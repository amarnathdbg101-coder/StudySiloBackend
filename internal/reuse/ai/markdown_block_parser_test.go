package ai

import (
	"testing"
	"time"
)

func TestParseMarkdownBlocks_Comprehensive(t *testing.T) {
	sampleMarkdown := `# Quantum Mechanics & Wave-Particle Duality

## Foundation & Principles
Matter exhibits both wave-like and particle-like properties.

### De Broglie Relation
The de Broglie wavelength is given by:
$$ \lambda = \frac{h}{p} $$

> [!TIP]
> 🎯 Exam Tip: Examiners frequently test the relation between momentum and kinetic energy: $p = \sqrt{2mE}$.

- [x] Verified Planck constant $h = 6.626 \times 10^{-34} \text{ J}\cdot\text{s}$
- [ ] Calculate relativistic mass correction

1. Determine kinetic energy of electron.
2. Substitute into de Broglie equation.

| Property | Photon | Electron |
|---|---|---|
| Rest Mass | 0 | 9.1e-31 kg |
| Charge | Neutral | -1.6e-19 C |

❌ Common Mistake: Assuming massive particles cannot diffract through crystal lattices.
✅ Correct: Davisson-Germer experiment proved electron diffraction through nickel crystals.

` + "```python\n# Calculate de Broglie wavelength\ndef wavelength(p):\n    return 6.626e-34 / p\n```"

	start := time.Now()
	blocks := ParseMarkdownBlocks(sampleMarkdown)
	elapsed := time.Since(start)

	t.Logf("Parsed %d blocks in %v", len(blocks), elapsed)
	if elapsed > 20*time.Millisecond {
		t.Errorf("expected parsing in <20ms, took %v", elapsed)
	}

	if len(blocks) < 10 {
		t.Fatalf("expected at least 10 blocks, got %d", len(blocks))
	}

	blockTypes := make(map[string]int)
	for _, b := range blocks {
		blockTypes[b.Type]++
	}

	expectedTypes := []string{"heading1", "heading3", "paragraph", "formula", "callout", "checklist", "numbered", "table", "comparison", "code"}
	for _, expected := range expectedTypes {
		if blockTypes[expected] == 0 {
			t.Errorf("expected block type %q to be present, but was missing", expected)
		}
	}
}
