package ai

import (
	"strings"
	"sync"
	"unicode/utf8"
)

// StreamChunkBuffer ensures that streaming chunks sent to SSE clients always end on complete,
// valid UTF-8 rune boundaries. This completely prevents broken multibyte characters (such as
// Hindi/Hinglish characters or mathematical symbols like ≈, θ, ∫) from causing client-side decode errors.
type StreamChunkBuffer struct {
	mu      sync.Mutex
	buffer  strings.Builder
	onFlush func(string)
}

// NewStreamChunkBuffer creates a new rune-safe stream buffer with the given flush callback.
func NewStreamChunkBuffer(onFlush func(string)) *StreamChunkBuffer {
	return &StreamChunkBuffer{
		onFlush: onFlush,
	}
}

// Write processes incoming text chunk, buffering incomplete runes and flushing valid text.
func (b *StreamChunkBuffer) Write(chunk string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer.WriteString(chunk)
	s := b.buffer.String()
	if len(s) == 0 {
		return
	}

	raw := []byte(s)
	cut := len(raw)

	// Check up to 3 trailing bytes for incomplete multi-byte sequence
	for i := 0; i < 4 && i < len(raw); i++ {
		idx := len(raw) - 1 - i
		if utf8.RuneStart(raw[idx]) {
			r, size := utf8.DecodeRune(raw[idx:])
			if r == utf8.RuneError && size == 1 && idx+size < len(raw)+1 {
				// Incomplete rune detected at end of buffer: hold back until next chunk
				cut = idx
			}
			break
		}
	}

	if cut > 0 {
		flushable := string(raw[:cut])
		b.buffer.Reset()
		if cut < len(raw) {
			b.buffer.Write(raw[cut:])
		}
		if b.onFlush != nil && len(flushable) > 0 {
			b.onFlush(flushable)
		}
	}
}

// Flush dispatches any remaining buffered content to the callback.
func (b *StreamChunkBuffer) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.buffer.Len() > 0 && b.onFlush != nil {
		b.onFlush(b.buffer.String())
		b.buffer.Reset()
	}
}
