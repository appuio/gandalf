package renderer

import (
	"fmt"
	"io"
	"strings"

	"github.com/appuio/gandalf/pkg/executor"
)

type Renderer struct {
	Matcher       *executor.Matcher
	Formatter     Formatter
	DocumentTitle string
	Out           io.Writer
}

func (r *Renderer) Render() error {
	steps, err := r.Matcher.PreparedSteps()
	if err != nil {
		return fmt.Errorf("failed to get prepared steps: %w", err)
	}

	title := r.DocumentTitle
	if title == "" {
		title = "Workflow"
	}
	r.write(r.Formatter.H1(title))

	for i, step := range steps {
		link := r.Formatter.SectionLink(step.Match, fmt.Sprintf("step-%d", i+1))
		if strings.HasPrefix(strings.ToLower(step.Match), "and") {
			link = "&nbsp;&nbsp;" + link
		}
		r.write(r.Formatter.ListItem(link))
	}
	r.write("\n")

	for i, step := range steps {
		r.write(r.Formatter.AddSectionID(r.Formatter.H2(step.Match), fmt.Sprintf("step-%d", i+1)))
		r.write(r.Formatter.Text(step.MatchedStep.Description))

		if len(step.MatchedStep.Inputs) > 0 {
			r.write(r.Formatter.H3("Inputs"))
			for _, input := range step.MatchedStep.Inputs {
				t := r.Formatter.InlineCode(input.Name)
				if input.Description != "" {
					t += ": " + r.Formatter.Text(input.Description)
				}
				r.write(r.Formatter.ListItem(t))
			}
			r.write("\n")
		}

		if len(step.MatchedStep.Outputs) > 0 {
			r.write(r.Formatter.H3("Outputs"))
			for _, output := range step.MatchedStep.Outputs {
				t := r.Formatter.InlineCode(output.Name)
				if output.Description != "" {
					t += ": " + r.Formatter.Text(output.Description)
				}
				r.write(r.Formatter.ListItem(t))
			}
			r.write("\n")
		}

		if step.MatchedStep.Run != "" {
			script := new(strings.Builder)

			script.WriteString("OUTPUT=$(mktemp)\n\n")
			for _, output := range step.MatchedStep.Inputs {
				script.WriteString(fmt.Sprintf("# export INPUT_%s=\n", output.Name))
			}
			script.WriteString("\n")

			script.WriteString(step.MatchedStep.Run)
			script.WriteString("\n\n")

			script.WriteString("# echo \"# Outputs\"\n")
			script.WriteString("# cat \"$OUTPUT\"\n")
			script.WriteString("# rm -f \"$OUTPUT\"\n")

			r.write(r.Formatter.H3("Script"))
			r.write(r.Formatter.CodeBlock(script.String()))
		}
	}

	return nil
}

func (r *Renderer) write(text string) {
	io.WriteString(r.Out, text)
}

type Formatter interface {
	H1(text string) string
	H2(text string) string
	H3(text string) string
	ListItem(text string) string
	Text(text string) string
	InlineCode(code string) string
	CodeBlock(code string) string
	AddSectionID(title, id string) string
	SectionLink(title, id string) string
}

type ASCIIDocFormatter struct{}

func (ASCIIDocFormatter) H1(text string) string {
	return fmt.Sprintf("= %s\n\n", text)
}

func (ASCIIDocFormatter) H2(text string) string {
	return fmt.Sprintf("== %s\n\n", text)
}

func (ASCIIDocFormatter) H3(text string) string {
	return fmt.Sprintf("=== %s\n\n", text)
}

func (ASCIIDocFormatter) ListItem(text string) string {
	return fmt.Sprintf("* %s\n", text)
}

func (f ASCIIDocFormatter) Text(text string) string {
	return fmt.Sprintf("%s\n\n", f.separateListsInText(text))
}

func (ASCIIDocFormatter) InlineCode(code string) string {
	return fmt.Sprintf("`%s`", code)
}

func (ASCIIDocFormatter) CodeBlock(code string) string {
	return fmt.Sprintf("[source,bash]\n----\n%s\n----\n\n", code)
}

func (ASCIIDocFormatter) AddSectionID(title, id string) string {
	return fmt.Sprintf("[[%s]]\n%s", id, title)
}

func (ASCIIDocFormatter) SectionLink(title, id string) string {
	return fmt.Sprintf("<<%s,%s>>", id, title)
}

// separateListsInText adds newlines before and after lists in the given text.
func (ASCIIDocFormatter) separateListsInText(text string) string {
	inList := false
	result := new(strings.Builder)

	for line := range strings.Lines(text) {
		curInList := strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || (strings.HasPrefix(line, " ") && inList)
		if curInList != inList {
			result.WriteString("\n")
			inList = curInList
		}
		result.WriteString(line)
	}
	return result.String()
}

type MarkdownFormatter struct{}

func (MarkdownFormatter) H1(text string) string {
	return fmt.Sprintf("# %s\n\n", text)
}

func (MarkdownFormatter) H2(text string) string {
	return fmt.Sprintf("## %s\n\n", text)
}

func (MarkdownFormatter) H3(text string) string {
	return fmt.Sprintf("### %s\n\n", text)
}

func (MarkdownFormatter) ListItem(text string) string {
	return fmt.Sprintf("- %s\n", text)
}

func (MarkdownFormatter) Text(text string) string {
	return fmt.Sprintf("%s\n\n", text)
}

func (MarkdownFormatter) InlineCode(code string) string {
	return fmt.Sprintf("`%s`", code)
}

func (MarkdownFormatter) CodeBlock(code string) string {
	return fmt.Sprintf("```bash\n%s\n```\n\n", code)
}

func (MarkdownFormatter) AddSectionID(title, id string) string {
	return fmt.Sprintf("<a name=\"%s\"></a>\n%s", id, title)
}

func (MarkdownFormatter) SectionLink(title, id string) string {
	return fmt.Sprintf("[%s](#%s)", title, id)
}
