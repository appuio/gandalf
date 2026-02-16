package renderer_test

import (
	"embed"
	"encoding/json"
	"strings"
	"testing"

	"github.com/appuio/gandalf/pkg/executor"
	"github.com/appuio/gandalf/pkg/renderer"
	"github.com/appuio/gandalf/pkg/steps"
	"github.com/appuio/gandalf/pkg/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func Test_ASCIIDocFormatter_Text_separateListsInText(t *testing.T) {
	formatter := renderer.ASCIIDocFormatter{}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "This is a list:\n* Item 1\n* Item 2\nEnd of list.",
			expected: "This is a list:\n\n* Item 1\n* Item 2\n\nEnd of list.",
		},
		{
			input:    "No list here, just text.",
			expected: "No list here, just text.",
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := formatter.Text(test.input)
			assert.Equal(t, test.expected+"\n\n", result)
		})
	}
}

//go:embed testdata
var testdata embed.FS

func Test_Renderer(t *testing.T) {
	rawWF, err := testdata.ReadFile("testdata/test.workflow")
	assert.NoError(t, err)
	wf, err := workflow.UnmarshalWorkflow(rawWF)
	assert.NoError(t, err)

	rawYAMLSteps, err := testdata.ReadFile("testdata/test/steps.yml")
	assert.NoError(t, err)
	rawJSONSteps, err := yaml.YAMLToJSON(rawYAMLSteps)
	assert.NoError(t, err)
	parsedFile := &steps.StepsFile{}
	require.NoError(t, json.Unmarshal(rawJSONSteps, parsedFile))

	matcher := &executor.Matcher{
		Workflow: wf,
		Steps:    parsedFile.Steps,
	}
	require.NoError(t, matcher.Prepare())

	formatters := map[string]renderer.Formatter{
		"md":   renderer.MarkdownFormatter{},
		"adoc": renderer.ASCIIDocFormatter{},
	}
	for format := range formatters {
		t.Run(format, func(t *testing.T) {
			out := new(strings.Builder)
			ren := &renderer.Renderer{
				Matcher:   matcher,
				Formatter: formatters[format],
				Out:       out,
			}
			require.NoError(t, ren.Render())

			expected, err := testdata.ReadFile("testdata/test/output." + format)
			assert.NoError(t, err)

			t.Log("Full output:\n" + out.String())

			assert.Equal(t, strings.TrimSpace(string(expected)), strings.TrimSpace(out.String()))
		})
	}
}
