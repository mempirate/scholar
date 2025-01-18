package document

import "testing"

func TestFindTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "empty",
			content:  "",
			expected: "",
		},
		{
			name:     "simple",
			content:  "# Title\n",
			expected: "Title",
		},
		{
			name:     "empty title",
			content:  "#\n",
			expected: "",
		},
		{
			name:     "no title",
			content:  "content",
			expected: "",
		},
		{
			name:     "multiple titles",
			content:  "# Title 1\n# Title 2\n",
			expected: "Title 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc := &Document{
				Content: []byte(test.content),
			}

			title := doc.FindTitle()
			if title != test.expected {
				t.Errorf("unexpected title: %s", title)
			}
		})
	}
}
