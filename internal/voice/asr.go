package voice

import "context"

// Transcript is the ASR output used to drive Beckn intent mapping.
type Transcript struct {
	Text       string
	Language   string
	Confidence float64
}

// ASRProvider converts voice audio to text (Bhashini adapter in production).
type ASRProvider interface {
	Transcribe(ctx context.Context, audio []byte) (*Transcript, error)
}

// StubASRProvider returns a fixed Santali transcript for development.
type StubASRProvider struct {
	FixedText string
}

// Transcribe implements ASRProvider.
func (s StubASRProvider) Transcribe(_ context.Context, _ []byte) (*Transcript, error) {
	text := s.FixedText
	if text == "" {
		text = "ᱢᱮᱨᱟ ᱟᱹᱲᱟᱹ ᱢᱮᱱᱟᱜᱼᱟ"
	}
	return &Transcript{
		Text:       text,
		Language:   "sat",
		Confidence: 1.0,
	}, nil
}
