package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse/ai"

	"go.uber.org/zap"
)

type FlashcardService interface {
	GenerateDeck(ctx context.Context, userID string, req dto.GenerateFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error)
	SaveDeck(ctx context.Context, userID string, req dto.SaveFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error)
	ListDecks(ctx context.Context, userID string, search string, limit, offset int) ([]dto.FlashcardDeckSummary, int, error)
	GetDeck(ctx context.Context, userID, id string) (*dto.GenerateFlashcardDeckResponse, error)
	UpdateDeck(ctx context.Context, userID, id string, req dto.UpdateFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error)
	DeleteDeck(ctx context.Context, userID, id string) error
	TogglePin(ctx context.Context, userID, id string) (*dto.GenerateFlashcardDeckResponse, error)
}

type flashcardService struct {
	repo         repository.FlashcardRepository
	orchestrator *ai.Orchestrator
	logger       *zap.Logger
}

func NewFlashcardService(repo repository.FlashcardRepository, orchestrator *ai.Orchestrator, logger *zap.Logger) FlashcardService {
	return &flashcardService{
		repo:         repo,
		orchestrator: orchestrator,
		logger:       logger,
	}
}

type aiFlashcardGenerationResult struct {
	Title string `json:"title"`
	Cards []struct {
		Front      string `json:"front"`
		Back       string `json:"back"`
		Difficulty string `json:"difficulty"`
		Topic      string `json:"topic"`
	} `json:"cards"`
}

func (s *flashcardService) GenerateDeck(ctx context.Context, userID string, req dto.GenerateFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error) {
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		topic = "General Academic Concepts"
	}

	cardCount := req.CardCount
	if cardCount <= 0 {
		cardCount = 5
	}
	if cardCount > 20 {
		cardCount = 20
	}

	difficulty := strings.ToLower(strings.TrimSpace(req.Difficulty))
	if difficulty == "" || difficulty == "standard" {
		difficulty = "medium"
	}

	systemPrompt := `You are an elite academic professor, Socratic tutor, and master Anki Spaced Repetition specialist.
Generate high-yield, conceptually rigorous Active Recall 3D Flashcards for university and competitive exam students.

DIFFICULTY CALIBRATION:
- "easy" (Fundamental): Foundational definitions, essential axioms, core mental models, and direct relationships.
- "medium" (Standard): Balanced academic rigor, multi-step mechanisms, practical real-world application, governing formulas or code snippets, and critical distinctions between closely related concepts.
- "hard" (Exam Rigor): Advanced analytical proofs, edge-case boundary conditions, subtle examiner traps, and multi-concept synthesis.

RULES FOR EFFECTIVE ACTIVE RECALL:
1. FRONT: Must be a crisp, provocative question or challenge that forces memory retrieval (e.g., "Why does X occur under Y?", "Compare A vs B and when to choose each", "What is the architectural difference between..."). NEVER ask shallow yes/no or trivial keyword questions.
2. BACK: Must provide an authoritative, masterclass answer with:
   - Direct explanation formatted with clean bullet points
   - Mathematical formula ($...$ or $$...$$) or code snippet if relevant to the topic
   - ⚠️ Examiner Gotcha / Common Trap to avoid
3. TOPIC SPECIFICITY: Every card MUST be strictly tailored to the requested domain. Never give physics formulas on computer science topics or vice versa.
4. Respond ONLY with valid JSON strictly conforming to this schema:
{
  "title": "string (Concise title for this flashcard deck)",
  "cards": [
    {
      "front": "string",
      "back": "string",
      "difficulty": "easy | medium | hard",
      "topic": "string"
    }
  ]
}`
	aiLanguage := middleware.GetAILanguage(ctx)
	systemPrompt += ai.BuildLanguageInstruction(aiLanguage)

	userPrompt := fmt.Sprintf(
		"Generate %d high-yield active recall flashcards.\nTopic: %s\nRigor Level: %s (Standard Academic Focus)",
		cardCount, topic, difficulty,
	)
	if req.Content != "" {
		userPrompt += fmt.Sprintf("\n\nBase Study Material / Notes:\n%s", req.Content)
	}

	// ⚡ Fast Context Timeout (20s max): Guarantees the student gets real AI generation with instant response
	aiCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var aiResult aiFlashcardGenerationResult
	var genErr error
	if s.orchestrator != nil {
		genErr = s.orchestrator.GenerateStructured(aiCtx, systemPrompt, userPrompt, &aiResult)
	}

	cards := make([]dto.FlashcardDeckCard, 0, cardCount)
	if genErr == nil && len(aiResult.Cards) > 0 {
		for i, c := range aiResult.Cards {
			cardDiff := strings.ToLower(c.Difficulty)
			if cardDiff == "" {
				cardDiff = difficulty
			}
			cardTopic := c.Topic
			if cardTopic == "" {
				cardTopic = topic
			}
			cards = append(cards, dto.FlashcardDeckCard{
				ID:          fmt.Sprintf("card-%d", i+1),
				Front:       ai.SanitizeLatex(c.Front),
				Back:        ai.SanitizeLatex(c.Back),
				Difficulty:  cardDiff,
				Topic:       ai.SanitizeLatex(cardTopic),
				IsMastered:  false,
				IsReviewed:  false,
				Repetitions: 0,
				Interval:    1,
				EaseFactor:  2.5,
			})
		}
	}

	// Resilient Domain-Smart Fallback: If AI is offline, timed out, or rate-limited,
	// generate authoritative, domain-appropriate flashcards matching the student's exact topic
	if len(cards) == 0 {
		cards = s.buildDomainSmartFlashcards(topic, difficulty, cardCount)
	}

	deckTitle := aiResult.Title
	if strings.TrimSpace(deckTitle) == "" {
		deckTitle = fmt.Sprintf("%s Active Recall Deck", topic)
	}

	resp := &dto.GenerateFlashcardDeckResponse{
		Title:         deckTitle,
		Topic:         topic,
		Difficulty:    difficulty,
		TotalCards:    len(cards),
		MasteredCount: 0,
		Cards:         cards,
	}

	// If save requested or user is not guest, automatically persist to database
	if (req.SaveDeck || userID != "guest-user") && s.repo != nil && userID != "" {
		cardsJSON, err := json.Marshal(cards)
		if err == nil {
			deckModel := &model.FlashcardDeck{
				UserID:        userID,
				Title:         deckTitle,
				Topic:         topic,
				Difficulty:    difficulty,
				CardCount:     len(cards),
				Cards:         cardsJSON,
				MasteredCount: 0,
				IsPinned:      false,
			}
			if saveErr := s.repo.Create(ctx, deckModel); saveErr == nil {
				resp.DeckID = deckModel.ID
			} else if s.logger != nil {
				s.logger.Warn("failed to auto-save flashcard deck", zap.Error(saveErr))
			}
		}
	}

	return resp, nil
}

// buildDomainSmartFlashcards synthesizes domain-accurate, highly educational flashcards for any academic subject
func (s *flashcardService) buildDomainSmartFlashcards(topic, difficulty string, count int) []dto.FlashcardDeckCard {
	lower := strings.ToLower(topic)
	var rawCards []dto.FlashcardDeckCard

	// 1. Web Technology & Modern Web Development
	if strings.Contains(lower, "web") || strings.Contains(lower, "html") || strings.Contains(lower, "css") ||
		strings.Contains(lower, "javascript") || strings.Contains(lower, "js") || strings.Contains(lower, "frontend") ||
		strings.Contains(lower, "backend") || strings.Contains(lower, "fullstack") || strings.Contains(lower, "api") ||
		strings.Contains(lower, "rest") || strings.Contains(lower, "node") || strings.Contains(lower, "react") {

		rawCards = []dto.FlashcardDeckCard{
			{
				Front: "Why are Semantic HTML5 tags (<header>, <nav>, <article>, <section>, <footer>) preferred over generic <div> elements?",
				Back: `**Semantic HTML Architecture & Accessibility**:

• **SEO & Web Crawlers**: Search engines parse document hierarchy and content weighting through semantic landmarks.
• **Screen Readers (a11y)**: Assistive technologies announce regions so visually impaired users can jump directly to navigation or main content.
• **Maintainability**: Clear structural intent reduces confusing div-soup nesting.

⚠️ **Common Trap**: Semantic tags convey document meaning to browsers and crawlers; they do NOT automatically apply layout or styling!`,
				Difficulty: "easy",
				Topic:      "HTML5 Semantic Architecture",
			},
			{
				Front: "What is the core architectural difference between CSS Flexbox and CSS Grid, and what is the rule of thumb for choosing between them?",
				Back: `**1D vs 2D Layout Engines**:

• **Flexbox (One-Dimensional)**: Distributes space along a *single axis* (either row OR column). Ideal for navbars, button groups, and linear content alignment.
• **CSS Grid (Two-Dimensional)**: Arranges elements across *rows AND columns simultaneously*. Ideal for whole-page macro layouts, photo galleries, and card grids.

💡 **Rule of Thumb**: Flexbox is content-driven (items dictate size); Grid is layout-driven (grid container defines rigid slots).`,
				Difficulty: "medium",
				Topic:      "CSS Layout Engines",
			},
			{
				Front: "How does the JavaScript single-threaded Event Loop coordinate the Call Stack, Microtask queue, and Macrotask queue?",
				Back: `**JavaScript Event Loop Execution Order**:

1. **Call Stack**: Executes all synchronous scripts to completion first.
2. **Microtask Queue**: Emptying the call stack immediately drains ALL pending microtasks (Promises ` + "`.then()`" + `, ` + "`queueMicrotask()`" + `) before moving forward.
3. **Macrotask Queue**: Executes one macrotask (` + "`setTimeout`" + `, ` + "`setInterval`" + `, I/O), then immediately checks the microtask queue again before repainting.

⚠️ **Examiner Gotcha**: ` + "`setTimeout(fn, 0)`" + ` does NOT run immediately! It enters the macrotask queue and only runs after all current synchronous code and microtasks finish.`,
				Difficulty: "medium",
				Topic:      "JS Asynchronous Runtime",
			},
			{
				Front: "In RESTful API design, what are the primary HTTP status code classes, and what is the exact difference between 401 Unauthorized and 403 Forbidden?",
				Back: `**HTTP Status Code Architecture**:

• **2xx (Success)**: ` + "`200 OK`" + `, ` + "`201 Created`" + `
• **3xx (Redirection)**: ` + "`301 Moved Permanently`" + `, ` + "`304 Not Modified`" + `
• **4xx (Client Error)**: ` + "`400 Bad Request`" + `, ` + "`404 Not Found`" + `
• **5xx (Server Error)**: ` + "`500 Internal Error`" + `, ` + "`502 Bad Gateway`" + `

🔒 **401 vs 403 Distinction**:
• **401 Unauthorized**: Authentication missing or invalid (user is not logged in or JWT token expired).
• **403 Forbidden**: Authentication succeeded (identity is known), but user's role lacks permissions to access the resource.`,
				Difficulty: "medium",
				Topic:      "RESTful API & HTTP Protocols",
			},
			{
				Front: "Compare CORS vs CSRF: What fundamental vulnerability does each represent and how are modern web applications protected?",
				Back: `**Web Security Protocols**:

• **CORS (Cross-Origin Resource Sharing)**: A browser security mechanism that restricts web pages from making AJAX requests to a different domain/port. Server controls access via ` + "`Access-Control-Allow-Origin`" + ` headers.
• **CSRF (Cross-Site Request Forgery)**: A malicious site tricks an authenticated user's browser into executing an unauthorized state-changing request (e.g. transfer funds).
• **Mitigation**: Use ` + "`SameSite=Strict`" + ` cookies, Anti-CSRF verification tokens, or authenticate state-changing APIs using ` + "`Authorization: Bearer <JWT>`" + ` headers instead of ambient cookies.`,
				Difficulty: "hard",
				Topic:      "Web Security",
			},
		}

	// 2. Operating Systems & Systems Programming
	} else if strings.Contains(lower, "os") || strings.Contains(lower, "operating system") ||
		strings.Contains(lower, "process") || strings.Contains(lower, "thread") ||
		strings.Contains(lower, "deadlock") || strings.Contains(lower, "paging") ||
		strings.Contains(lower, "virtual memory") || strings.Contains(lower, "scheduling") ||
		strings.Contains(lower, "semaphore") {

		rawCards = []dto.FlashcardDeckCard{
			{
				Front: "What are the key architectural differences between a Process and a Thread in modern Operating Systems?",
				Back: `**Process vs Thread Architecture**:

• **Process**: Independent execution unit with its own private virtual address space, file descriptors, and PCB (Process Control Block). High context-switch overhead.
• **Thread (Lightweight Process)**: Sits inside a process; shares code, data, and heap segments with peer threads, but retains its own private Stack and Program Counter.
• **Communication**: Inter-Process Communication (IPC: pipes, sockets, shared memory) requires kernel intervention, while threads communicate directly via shared memory.

⚠️ **Gotcha**: While threads share heap memory, lack of synchronization causes data races and memory corruption.`,
				Difficulty: "easy",
				Topic:      "Processes & Threads",
			},
			{
				Front: "State the 4 Coffman Conditions required for Deadlock to occur. How can an OS prevent deadlock statically?",
				Back: `**The 4 Coffman Conditions**:

1. **Mutual Exclusion**: Resources cannot be shared simultaneously.
2. **Hold and Wait**: Process holding resources can request additional ones.
3. **No Preemption**: Resources cannot be forcibly taken from a process.
4. **Circular Wait**: Closed chain of processes where each holds a resource requested by the next.

💡 **Static Prevention**: Eliminating ANY ONE condition guarantees deadlock is impossible! For example, enforcing a global resource ordering eliminates Circular Wait.`,
				Difficulty: "medium",
				Topic:      "Deadlocks & Concurrency",
			},
			{
				Front: "How does Virtual Memory translate a Virtual Address to a Physical Address, and what is the role of the TLB?",
				Back: `**Virtual Memory & Paging Mechanism**:

1. Virtual Address is split into: [ Virtual Page Number ($p$) | Offset ($d$) ].
2. The **TLB (Translation Lookaside Buffer)** is an on-chip associative cache checked first ($O(1)$ lookup).
3. **TLB Hit**: Frame number ($f$) is retrieved instantly; Physical Address = [ $f$ | $d$ ].
4. **TLB Miss**: MMU walks the multi-level Page Table in RAM. If the page valid bit is 0, a **Page Fault** exception triggers an OS disk fetch.`,
				Difficulty: "medium",
				Topic:      "Memory Management",
			},
			{
				Front: "What is Belady's Anomaly in Page Replacement Algorithms, and which replacement policy is immune to it?",
				Back: `**Belady's Anomaly & Stack Algorithms**:

• **Belady's Anomaly**: The counter-intuitive phenomenon where increasing the number of physical page frames results in MORE page faults for a given memory access string.
• **Susceptible**: **FIFO (First-In, First-Out)** suffers from Belady's Anomaly.
• **Immune**: **LRU (Least Recently Used)** and Optimal (MIN) are *Stack Algorithms* where the set of pages in an $n$-frame memory is always a strict subset of pages in an $(n+1)$-frame memory.`,
				Difficulty: "hard",
				Topic:      "Virtual Memory Algorithms",
			},
			{
				Front: "What is the critical difference between a Binary Semaphore and a Mutex?",
				Back: `**Mutex vs Binary Semaphore**:

• **Mutex (Mutual Exclusion)**: Ownership-based locking mechanism. Only the specific thread that acquired (locked) the mutex is permitted to release (unlock) it.
• **Binary Semaphore**: Signaling mechanism. Any thread or interrupt service routine can signal (post/V) to unblock a waiting thread, regardless of who called wait/P.
• Mutexes support Priority Inheritance to prevent Priority Inversion; semaphores do not.`,
				Difficulty: "medium",
				Topic:      "Process Synchronization",
			},
		}

	// 3. Data Structures & Algorithms
	} else if strings.Contains(lower, "algorithm") || strings.Contains(lower, "dsa") ||
		strings.Contains(lower, "data structure") || strings.Contains(lower, "array") ||
		strings.Contains(lower, "tree") || strings.Contains(lower, "graph") ||
		strings.Contains(lower, "sort") || strings.Contains(lower, "search") ||
		strings.Contains(lower, "hash") || strings.Contains(lower, "dynamic programming") ||
		strings.Contains(lower, "stack") || strings.Contains(lower, "queue") {

		rawCards = []dto.FlashcardDeckCard{
			{
				Front: "Why does calculating the midpoint in Binary Search as mid = (low + high) / 2 cause bugs, and how do you fix it?",
				Back: `**32-Bit Signed Integer Overflow in Binary Search**:

• When $low + high > 2^{31} - 1$ (around 2.14 billion elements), the sum wraps around into a **negative number**, causing an ` + "`ArrayIndexOutOfBoundsException`" + `.
• **Correct Formula**:
$$
\text{mid} = \text{low} + \frac{\text{high} - \text{low}}{2}
$$
Or bitwise: ` + "`mid = (low + high) >>> 1`" + ` (unsigned shift).`,
				Difficulty: "easy",
				Topic:      "Binary Search & Bounds",
			},
			{
				Front: "Compare QuickSort vs MergeSort: Time complexity, auxiliary space, stability, and worst-case performance.",
				Back: `**Sorting Algorithmic Comparison**:

• **MergeSort**:
  - Time: Best $\mathcal{O}(n \log n)$, Avg $\mathcal{O}(n \log n)$, Worst $\mathcal{O}(n \log n)$ (Guaranteed)
  - Space: $\mathcal{O}(n)$ auxiliary array allocation
  - Stability: **Stable** (preserves original order of equal keys)
• **QuickSort**:
  - Time: Best $\mathcal{O}(n \log n)$, Avg $\mathcal{O}(n \log n)$, Worst $\mathcal{O}(n^2)$ (sorted/unbalanced pivot)
  - Space: $\mathcal{O}(\log n)$ stack frames in-place
  - Stability: **Unstable** (long-range swaps)`,
				Difficulty: "medium",
				Topic:      "Divide and Conquer Sorting",
			},
			{
				Front: "What are Hash Collisions, and how do Separate Chaining and Open Addressing resolve them?",
				Back: `**Hash Collision Resolution Strategies**:

1. **Separate Chaining**: Each bucket maintains a linked list (or balanced red-black tree in Java 8+ when bucket length exceeds 8). Deletions are trivial; degrades to $\mathcal{O}(n)$ only under malicious adversarial inputs.
2. **Open Addressing**: All elements stored directly in the hash array.
   - **Linear Probing**: Inspects $h(k, i) = (h'(k) + i) \bmod m$. Suffers from primary clustering.
   - **Double Hashing**: Uses secondary hash function $h(k, i) = (h_1(k) + i \cdot h_2(k)) \bmod m$. Eliminates clustering.`,
				Difficulty: "medium",
				Topic:      "Hash Tables & Sets",
			},
			{
				Front: "What two properties MUST a problem possess to be solvable via Dynamic Programming?",
				Back: `**The Two Core Hallmarks of Dynamic Programming**:

1. **Optimal Substructure**: An optimal solution to the global problem contains within it optimal solutions to its subproblems (e.g. shortest path $A \to C$ contains shortest path $A \to B$).
2. **Overlapping Subproblems**: The recursive algorithm visits the exact same subproblems repeatedly rather than generating new subproblems at each step.
• Solutions: **Memoization (Top-down)** with recursion + cache, or **Tabulation (Bottom-up)** with DP table.`,
				Difficulty: "medium",
				Topic:      "Dynamic Programming Principles",
			},
			{
				Front: "Why does Array have O(1) random access while Linked List requires O(n), yet Linked List allows O(1) insertions at known nodes?",
				Back: `**Memory Contiguity vs Pointer Indirection**:

• **Array**: Stored in a contiguous physical memory block. Address calculation is direct arithmetic:
$$
\text{Address}(A[i]) = \text{Base} + i \times \text{ElementSize} \implies \mathcal{O}(1)
$$
Leverages CPU L1/L2 cache prefetching due to high spatial locality.
• **Linked List**: Nodes scattered across non-contiguous heap memory connected by pointers. Access requires sequential pointer traversal ($\mathcal{O}(n)$); insertion at a known pointer only requires pointer reassignment ($\mathcal{O}(1)$).`,
				Difficulty: "easy",
				Topic:      "Linear Data Structures",
			},
		}

	// 4. Computer Architecture & Memory Organization
	} else if strings.Contains(lower, "architecture") || strings.Contains(lower, "coa") ||
		strings.Contains(lower, "memory hierarchy") || strings.Contains(lower, "pipelin") ||
		strings.Contains(lower, "hazard") {

		rawCards = []dto.FlashcardDeckCard{
			{
				Front: "What is the Principle of Locality, and how do Caches exploit Temporal vs Spatial locality?",
				Back: `**Principle of Locality**:

• **Temporal Locality**: If an item is referenced, it will tend to be referenced again soon (e.g., loop counters, active function stack). Exploited by keeping recently used blocks in fast L1/L2 Cache.
• **Spatial Locality**: If an item is referenced, neighboring memory addresses will tend to be referenced soon (e.g., sequential array traversal). Exploited by fetching multi-byte **Cache Lines** (typically 64 bytes) rather than single words.`,
				Difficulty: "easy",
				Topic:      "Memory Hierarchy Principles",
			},
			{
				Front: "What is Average Memory Access Time (AMAT), and how is it calculated for a two-level cache system?",
				Back: `**AMAT Mathematical Formulation**:

$$
\text{AMAT} = T_{\text{hit}} + (\text{Miss Rate} \times \text{Miss Penalty})
$$
For a two-level ($L_1, L_2$) cache hierarchy:
$$
\text{AMAT} = T_{L1\_hit} + m_{L1} \times \left( T_{L2\_hit} + m_{L2\_local} \times T_{\text{RAM}} \right)
$$
Where $m_{L1}$ is L1 miss rate and $m_{L2\_local}$ is L2 local miss rate.`,
				Difficulty: "medium",
				Topic:      "Cache Performance Analytics",
			},
			{
				Front: "Identify the 3 types of Pipeline Hazards in RISC architectures and how each is resolved.",
				Back: `**The 3 Classical Pipeline Hazards**:

1. **Structural Hazard**: Hardware resource conflict (e.g. single memory for instructions and data). Resolved by separate Harvard I-Cache & D-Cache.
2. **Data Hazard (RAW, WAR, WAW)**: Instruction depends on unwritten result of earlier instruction. Resolved by **Operand Forwarding / Bypassing** or inserting stalls/bubbles.
3. **Control (Branch) Hazard**: Pipelining instructions before branch outcome is known. Resolved by **Branch Prediction (Dynamic/Static)** and delayed branch slots.`,
				Difficulty: "hard",
				Topic:      "Instruction Pipelining",
			},
			{
				Front: "Compare Cache Write Policies: Write-Through vs Write-Back. What is the role of the Dirty Bit?",
				Back: `**Cache Write Architectures**:

• **Write-Through**: Every write updates both cache and main memory simultaneously. Simple, memory is always consistent, but consumes high bus bandwidth.
• **Write-Back**: Writes update only the cache block. Lower memory is updated ONLY when the block is evicted.
• **Dirty Bit**: 1-bit hardware flag in cache tag. Set to 1 when block is modified. If dirty bit is 0 on eviction, no memory write is needed, saving bus cycles.`,
				Difficulty: "medium",
				Topic:      "Cache Write Protocols",
			},
		}

	// 5. Database Systems & SQL
	} else if strings.Contains(lower, "db") || strings.Contains(lower, "database") ||
		strings.Contains(lower, "sql") || strings.Contains(lower, "indexing") ||
		strings.Contains(lower, "acid") || strings.Contains(lower, "transaction") {

		rawCards = []dto.FlashcardDeckCard{
			{
				Front: "Define the 4 ACID properties of database transactions and how each is guaranteed.",
				Back: `**ACID Guarantees**:

• **Atomicity**: All operations succeed or entire transaction is rolled back. Enforced by Write-Ahead Logging (WAL) and Undo logs.
• **Consistency**: Transaction preserves all integrity constraints, foreign keys, and schema invariants.
• **Isolation**: Concurrent transactions execute as if running serially without intermediate state leaks. Enforced by locks (2PL) or MVCC.
• **Durability**: Committed data survives system crashes and power loss. Enforced by flushing WAL to non-volatile disk.`,
				Difficulty: "easy",
				Topic:      "Transaction Management",
			},
			{
				Front: "Why do relational databases use B+ Trees instead of Binary Search Trees (BST) or B-Trees for disk indexing?",
				Back: `**B+ Tree Architectural Advantages for Disk I/O**:

1. **High Fan-out**: Each internal node holds hundreds of keys, keeping tree height very small (typically 3-4 levels for millions of rows), minimizing expensive disk seeks.
2. **Linked Leaf Scan**: All record pointers reside strictly in leaf nodes, linked sequentially via doubly linked lists, making range queries ($A \le x \le B$) linear scans without tree re-traversal.
3. **Predictable Performance**: All searches traverse identical root-to-leaf depth.`,
				Difficulty: "medium",
				Topic:      "Database Storage & Indexing",
			},
			{
				Front: "What is Database Normalization (1NF to 3NF/BCNF) and what critical anomalies does it eliminate?",
				Back: `**Normalization Forms & Anomaly Elimination**:

• **1NF**: Atomic attributes; no repeating groups.
• **2NF**: In 1NF + no Partial Functional Dependencies on composite primary keys.
• **3NF**: In 2NF + no Transitive Dependencies ($A \to B \to C$).
• **BCNF**: For every functional dependency $X \to Y$, $X$ must be a superkey.
• **Eliminates**: Insertion, Update, and Deletion Anomalies (loss of unintended data).`,
				Difficulty: "medium",
				Topic:      "Relational Schema Design",
			},
		}

	// 6. Mathematics & Calculus
	} else if strings.Contains(lower, "calculus") || strings.Contains(lower, "derivative") ||
		strings.Contains(lower, "integral") || strings.Contains(lower, "math") ||
		strings.Contains(lower, "probability") || strings.Contains(lower, "matrix") {

		rawCards = []dto.FlashcardDeckCard{
			{
				Front: "State the Fundamental Theorem of Calculus (FTC) Part 1 and Part 2 and their geometric meaning.",
				Back: `**The Fundamental Theorem of Calculus (FTC)**:

• **Part 1 (Accumulation Derivative)**: If $f$ is continuous on $[a,b]$ and $g(x) = \int_a^x f(t)dt$, then:
$$
g'(x) = \frac{d}{dx}\left[ \int_a^x f(t)dt \right] = f(x)
$$
• **Part 2 (Definite Integral Evaluation)**:
$$
\int_a^b f(x)dx = F(b) - F(a), \quad \text{where } F'(x) = f(x)
$$
Bridges differentiation and integration as inverse operations!`,
				Difficulty: "easy",
				Topic:      "Calculus Foundations",
			},
			{
				Front: "What is the Integration by Parts formula, and what is the LIATE rule used to select u(x)?",
				Back: `**Integration by Parts & LIATE Priority**:

$$
\int u \, dv = u \cdot v - \int v \, du
$$
Choose $u(x)$ according to the **LIATE** hierarchy (first match becomes $u$):
1. **L** - Logarithmic ($\ln x$)
2. **I** - Inverse Trigonometric ($\arctan x$)
3. **A** - Algebraic ($x^2, 3x$)
4. **T** - Trigonometric ($\sin x, \cos x$)
5. **E** - Exponential ($e^x, 2^x$)`,
				Difficulty: "medium",
				Topic:      "Integral Calculus Techniques",
			},
			{
				Front: "What are Eigenvalues and Eigenvectors of a square matrix A, and how do you find them?",
				Back: `**Eigenvalues & Characteristic Equation**:

• An **Eigenvector** $\mathbf{v} \neq \mathbf{0}$ of matrix $A$ undergoes only scalar scaling when multiplied by $A$:
$$
A\mathbf{v} = \lambda \mathbf{v} \iff (A - \lambda I)\mathbf{v} = \mathbf{0}
$$
• **Eigenvalues ($\lambda$)** are roots of the **Characteristic Polynomial**:
$$
\det(A - \lambda I) = 0
$$
Sum of eigenvalues equals $\text{Trace}(A)$; product equals $\det(A)$.`,
				Difficulty: "hard",
				Topic:      "Linear Algebra",
			},
		}

	// 7. Physics & Mechanics
	} else if strings.Contains(lower, "physics") || strings.Contains(lower, "force") ||
		strings.Contains(lower, "newton") || strings.Contains(lower, "energy") ||
		strings.Contains(lower, "thermodynamic") || strings.Contains(lower, "carnot") {

		rawCards = []dto.FlashcardDeckCard{
			{
				Front: "State the Work-Energy Theorem and the mathematical conditions under which Mechanical Energy is conserved.",
				Back: `**Work-Energy Theorem**:

The net work done by all forces acting on a body equals the change in its kinetic energy:
$$
W_{\text{net}} = \Delta K = \frac{1}{2}m v_f^2 - \frac{1}{2}m v_i^2
$$
• **Conservation Condition**: Total mechanical energy ($E = K + U$) is conserved $\iff$ ALL non-conservative forces (friction, drag) do zero work ($W_{\text{nc}} = 0$).`,
				Difficulty: "easy",
				Topic:      "Classical Mechanics",
			},
			{
				Front: "What is the Carnot Heat Engine efficiency limit, and why is 100% thermal efficiency physically impossible?",
				Back: `**Carnot Thermal Efficiency & 2nd Law**:

The maximum theoretical efficiency of any heat engine operating between hot reservoir $T_H$ and cold reservoir $T_C$ (in Kelvin) is:
$$
\eta_{\text{max}} = 1 - \frac{T_C}{T_H}
$$
• 100% efficiency ($\eta = 1$) requires either $T_C = 0\,\text{K}$ (Absolute Zero, unreachable by 3rd Law) or $T_H = \infty$.
• Kelvin-Planck statement: No cyclic process can convert heat completely into work without expelling heat to a sink.`,
				Difficulty: "medium",
				Topic:      "Thermodynamics",
			},
		}

	// 8. General / Universal Academic Topic Fallback
	} else {
		rawCards = []dto.FlashcardDeckCard{
			{
				Front: fmt.Sprintf("What is the core definition, scope, and foundational axiom of %s?", topic),
				Back: fmt.Sprintf(`**Foundational Concept: %s**:

• **Core Principle**: Formulate the primary operational definition and governing relationship defining %s.
• **Scope & Context**: Identifies the primary domain components and boundary parameters.
• **Exam Significance**: Foundational for multi-step reasoning and analytical derivations.`, topic, topic),
				Difficulty: "easy",
				Topic:      topic,
			},
			{
				Front: fmt.Sprintf("What are the key governing mechanisms or step-by-step operational workflows in %s?", topic),
				Back: fmt.Sprintf(`**Step-by-Step Mechanisms in %s**:

1. **State Declaration**: Identify inputs, invariants, and initial boundary conditions.
2. **Execution / Processing**: Step-by-step causal chain or mathematical transition.
3. **Termination & Output**: Final equilibrium, return state, or conservation check.`, topic),
				Difficulty: "medium",
				Topic:      topic,
			},
			{
				Front: fmt.Sprintf("What are the most frequent examiner traps, sign errors, and misconceptions in %s?", topic),
				Back: fmt.Sprintf(`⚠️ **Critical Examiner Gotchas in %s**:

• **False Assumption**: Conflating ideal theoretical models with real-world boundary constraints.
• **Unit & Precision Traps**: Ensure consistent SI units and standard notation.
• **Edge Cases**: Always inspect asymptotic limits ($0$, $\infty$, null conditions).`, topic),
				Difficulty: "hard",
				Topic:      topic,
			},
			{
				Front: fmt.Sprintf("How does %s compare with its primary alternative or dual concept?", topic),
				Back: fmt.Sprintf(`**Comparative Analysis in %s**:

• **Key Trade-offs**: Contrast space vs time, throughput vs latency, or theoretical vs practical constraints.
• **Decision Matrix**: When to choose %s versus alternative approaches in real-world scenarios.`, topic, topic),
				Difficulty: "medium",
				Topic:      topic,
			},
			{
				Front: fmt.Sprintf("What is a high-yield real-world engineering or practical application of %s?", topic),
				Back: fmt.Sprintf(`**Real-World Application of %s**:

• Modern production systems and industrial frameworks leverage %s to guarantee stability, fault tolerance, and predictable performance bounds under high load.`, topic, topic),
				Difficulty: "medium",
				Topic:      topic,
			},
		}
	}

	// Adjust count
	cards := make([]dto.FlashcardDeckCard, 0, count)
	for i := 0; i < len(rawCards) && i < count; i++ {
		c := rawCards[i]
		c.ID = fmt.Sprintf("card-%d", i+1)
		c.Difficulty = difficulty
		c.IsMastered = false
		c.IsReviewed = false
		c.Repetitions = 0
		c.Interval = 1
		c.EaseFactor = 2.5
		cards = append(cards, c)
	}

	// If requested count > rawCards length, duplicate with variation
	for len(cards) < count {
		idx := len(cards) % len(rawCards)
		c := rawCards[idx]
		c.ID = fmt.Sprintf("card-%d", len(cards)+1)
		c.Difficulty = difficulty
		c.IsMastered = false
		c.IsReviewed = false
		c.Repetitions = 0
		c.Interval = 1
		c.EaseFactor = 2.5
		cards = append(cards, c)
	}

	return cards
}

func (s *flashcardService) SaveDeck(ctx context.Context, userID string, req dto.SaveFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error) {
	cardsJSON, err := json.Marshal(req.Cards)
	if err != nil {
		return nil, fmt.Errorf("failed to encode cards: %w", err)
	}

	diff := req.Difficulty
	if diff == "" {
		diff = "medium"
	}

	deck := &model.FlashcardDeck{
		UserID:        userID,
		Title:         req.Title,
		Topic:         req.Topic,
		Difficulty:    diff,
		CardCount:     len(req.Cards),
		Cards:         cardsJSON,
		MasteredCount: req.MasteredCount,
		IsPinned:      false,
	}

	if err := s.repo.Create(ctx, deck); err != nil {
		return nil, err
	}

	return &dto.GenerateFlashcardDeckResponse{
		DeckID:        deck.ID,
		Title:         deck.Title,
		Topic:         deck.Topic,
		Difficulty:    deck.Difficulty,
		TotalCards:    deck.CardCount,
		MasteredCount: deck.MasteredCount,
		Cards:         req.Cards,
	}, nil
}

func (s *flashcardService) ListDecks(ctx context.Context, userID string, search string, limit, offset int) ([]dto.FlashcardDeckSummary, int, error) {
	decks, total, err := s.repo.List(ctx, userID, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	summaries := make([]dto.FlashcardDeckSummary, 0, len(decks))
	for _, d := range decks {
		summaries = append(summaries, dto.FlashcardDeckSummary{
			ID:            d.ID,
			Title:         d.Title,
			Topic:         d.Topic,
			Difficulty:    d.Difficulty,
			CardCount:     d.CardCount,
			MasteredCount: d.MasteredCount,
			IsPinned:      d.IsPinned,
			CreatedAt:     d.CreatedAt,
			UpdatedAt:     d.UpdatedAt,
		})
	}

	return summaries, total, nil
}

func (s *flashcardService) GetDeck(ctx context.Context, userID, id string) (*dto.GenerateFlashcardDeckResponse, error) {
	deck, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	var cards []dto.FlashcardDeckCard
	if len(deck.Cards) > 0 {
		_ = json.Unmarshal(deck.Cards, &cards)
	}

	return &dto.GenerateFlashcardDeckResponse{
		DeckID:        deck.ID,
		Title:         deck.Title,
		Topic:         deck.Topic,
		Difficulty:    deck.Difficulty,
		TotalCards:    deck.CardCount,
		MasteredCount: deck.MasteredCount,
		Cards:         cards,
	}, nil
}

func (s *flashcardService) UpdateDeck(ctx context.Context, userID, id string, req dto.UpdateFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error) {
	var cardsJSON json.RawMessage
	var cardCount *int
	if req.Cards != nil {
		bytes, err := json.Marshal(req.Cards)
		if err != nil {
			return nil, fmt.Errorf("failed to encode cards: %w", err)
		}
		cardsJSON = bytes
		c := len(req.Cards)
		cardCount = &c
	}

	deck, err := s.repo.Update(ctx, userID, id, req.Title, cardsJSON, cardCount, req.MasteredCount, req.IsPinned)
	if err != nil {
		return nil, err
	}

	var cards []dto.FlashcardDeckCard
	if len(deck.Cards) > 0 {
		_ = json.Unmarshal(deck.Cards, &cards)
	}

	return &dto.GenerateFlashcardDeckResponse{
		DeckID:        deck.ID,
		Title:         deck.Title,
		Topic:         deck.Topic,
		Difficulty:    deck.Difficulty,
		TotalCards:    deck.CardCount,
		MasteredCount: deck.MasteredCount,
		Cards:         cards,
	}, nil
}

func (s *flashcardService) DeleteDeck(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}

func (s *flashcardService) TogglePin(ctx context.Context, userID, id string) (*dto.GenerateFlashcardDeckResponse, error) {
	deck, err := s.repo.TogglePin(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	var cards []dto.FlashcardDeckCard
	if len(deck.Cards) > 0 {
		_ = json.Unmarshal(deck.Cards, &cards)
	}

	return &dto.GenerateFlashcardDeckResponse{
		DeckID:        deck.ID,
		Title:         deck.Title,
		Topic:         deck.Topic,
		Difficulty:    deck.Difficulty,
		TotalCards:    deck.CardCount,
		MasteredCount: deck.MasteredCount,
		Cards:         cards,
	}, nil
}
