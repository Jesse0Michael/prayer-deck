package media

import (
	"testing"

	"github.com/jesse0michael/pkg/test"
	"github.com/stretchr/testify/require"
)

// renditionSummary drops encoded bytes, which vary with the encoder, from comparisons.
type renditionSummary struct {
	Name        string
	ContentType string
	Extension   string
	Width       int
	Height      int
}

func TestRenditions(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		expected  []renditionSummary
		wantError bool
	}{
		{
			name: "small image is never enlarged",
			file: "testdata/small.png",
			expected: []renditionSummary{
				{Name: "original", ContentType: "image/png", Extension: "png", Width: 4, Height: 6},
				{Name: "display", ContentType: "image/jpeg", Extension: "jpg", Width: 4, Height: 6},
				{Name: "thumbnail", ContentType: "image/jpeg", Extension: "jpg", Width: 4, Height: 6},
			},
		},
		{
			name: "large transparent image is scaled to fit each rendition",
			file: "testdata/large.png",
			expected: []renditionSummary{
				{Name: "original", ContentType: "image/png", Extension: "png", Width: 2000, Height: 3000},
				{Name: "display", ContentType: "image/jpeg", Extension: "jpg", Width: 1066, Height: 1600},
				{Name: "thumbnail", ContentType: "image/jpeg", Extension: "jpg", Width: 320, Height: 480},
			},
		},
		{
			name:      "not an image",
			file:      "testdata/not-image.txt",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renditions, err := Renditions(test.LoadFile(t, tt.file))

			var summaries []renditionSummary
			for _, r := range renditions {
				require.NotEmpty(t, r.Data)
				summaries = append(summaries, renditionSummary{Name: r.Name, ContentType: r.ContentType, Extension: r.Extension, Width: r.Width, Height: r.Height})
			}
			require.Equal(t, tt.expected, summaries)
			require.Equal(t, tt.wantError, err != nil)
		})
	}
}
