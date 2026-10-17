-- Migration 000011: Create Exam Curricula Schema
-- Stores high-yield syllabus topics, exam keywords, and trap areas in PostgreSQL

CREATE TABLE IF NOT EXISTS exam_curricula (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_code VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    keywords TEXT[] NOT NULL DEFAULT '{}',
    high_yield_topics TEXT[] NOT NULL DEFAULT '{}',
    common_traps TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_exam_curricula_code ON exam_curricula (exam_code);

-- Seed initial standardized academic curricula
INSERT INTO exam_curricula (exam_code, display_name, keywords, high_yield_topics, common_traps)
VALUES
(
    'gate_cs',
    'GATE Computer Science & IT',
    ARRAY['gate', 'computer', 'tech', 'code', 'coding', 'software', 'cs', 'it'],
    ARRAY[
        'Data Structures & Algorithms',
        'Operating Systems & Concurrency',
        'Computer Networks & Protocols',
        'Database Management & Indexing',
        'Computer Architecture & Pipelining',
        'Theory of Computation & Automata'
    ],
    ARRAY['Deadlocks vs Starvation', 'B+ Tree Order vs Pointers', 'Pipelining Data Hazards', 'TCP Congestion Window Transitions']
),
(
    'neet_medical',
    'NEET Medical Entrance (UG)',
    ARRAY['neet', 'medical', 'biology', 'mbbs', 'doctor', 'usmle'],
    ARRAY[
        'Molecular Basis of Inheritance',
        'Human Physiology & Circulation',
        'Genetics & Mendelian Principles',
        'Organic Reaction Mechanisms',
        'Chemical Equilibrium & Thermodynamics',
        'Cell Structure & Division'
    ],
    ARRAY['Mendelian Linkage Exceptions', 'Cardiac Cycle Pressure Timing', 'Le Chatelier Equilibrium Shifts', 'Enzyme Inhibition Kinetics']
),
(
    'jee_engineering',
    'JEE Main & Advanced (Engineering)',
    ARRAY['jee', 'iit', 'bitsat', 'engineering', 'physics', 'math'],
    ARRAY[
        'Rotational Dynamics & Inertia',
        'Electromagnetic Induction & Optics',
        'Definite Integrals & Calculus',
        'Thermodynamics & Heat Cycles',
        'Matrices & Linear Transformations',
        'Chemical Kinetics & Equilibrium'
    ],
    ARRAY['Moment of Inertia Parallel Axis Sign', 'Lenz Law Induced EMF Signs', 'Definite Integral Discontinuity Traps', 'Carnot Efficiency Limits']
),
(
    'cbse_school',
    'CBSE / ICSE School Boards (Class 10 & 12)',
    ARRAY['class 10', 'class 12', 'cbse', 'icse', 'board', 'school'],
    ARRAY[
        'Electricity & Ohm''s Law',
        'Light: Reflection & Refraction',
        'Carbon & Its Chemical Compounds',
        'Life Processes & Circulation',
        'Quadratic Equations & Trigonometry',
        'Acids, Bases & Salts'
    ],
    ARRAY['Series vs Parallel Resistance Power Dissipation', 'Lens Formula Sign Convention', 'Homologous Series Functional Groups', 'pH Scale Logarithmic Shifts']
),
(
    'upsc_civil',
    'UPSC Civil Services Examination',
    ARRAY['upsc', 'civil', 'ias', 'ips', 'humanities', 'history', 'polity'],
    ARRAY[
        'Indian Constitutional Framework & Rights',
        'Monsoon Dynamics & Physical Geography',
        'Macroeconomics, Inflation & Monetary Policy',
        'Modern Indian Freedom Movement',
        'International Relations & Multilateral Treaties',
        'Environmental Ecology & Biodiversity'
    ],
    ARRAY['Fundamental Rights vs DPSP Precedence', 'El Nino vs Indian Ocean Dipole Patterns', 'Repo Rate vs Reverse Repo Impact on Money Supply', 'Ramsar Wetland Classification']
),
(
    'general',
    'General Academic Studies',
    ARRAY['general', 'academic', 'studies', 'college', 'foundation'],
    ARRAY[
        'Active Recall & Spaced Repetition',
        'Critical Thinking & Logic Proofs',
        'Scientific Method & Data Analysis',
        'Core Mathematical Foundations',
        'Thermodynamics Principles',
        'Probability & Applied Statistics'
    ],
    ARRAY['Correlation vs Causation', 'Bayes Theorem Prior Probability Fallacy', 'Entropy Increase in Closed Systems']
)
ON CONFLICT (exam_code) DO NOTHING;
