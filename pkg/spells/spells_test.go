package spells

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_VariableType_Json_Regular(t *testing.T) {
	for _, str := range []string{
		"{\"name\": \"MyVar\"}",
		"{\"name\": \"MyVar\", \"type\":\"regular\"}",
		"{\"name\": \"MyVar\", \"type\":\"\"}",
	} {
		input := Input{}

		err := json.Unmarshal([]byte(str), &input)
		assert.NoError(t, err)

		assert.True(t, input.Type.IsRegular())
		assert.False(t, input.Type.IsLocal())
		assert.False(t, input.Type.IsSensitive())

		nbytes, err := json.Marshal(input)
		nstr := string(nbytes)

		assert.NoError(t, err)

		assert.NotContains(t, nstr, "local")
		assert.NotContains(t, nstr, "sensitive")
		assert.NotContains(t, nstr, "regular")
	}
}

func Test_VariableType_Json_Local(t *testing.T) {

	str := []byte("{\"name\": \"MyVar\", \"type\":\"local\"}")

	input := Input{}

	err := json.Unmarshal(str, &input)

	assert.NoError(t, err)

	assert.False(t, input.Type.IsRegular())
	assert.True(t, input.Type.IsLocal())
	assert.False(t, input.Type.IsSensitive())

	nbytes, err := json.Marshal(input)
	nstr := string(nbytes)

	assert.NoError(t, err)

	assert.Contains(t, nstr, "local")
	assert.NotContains(t, nstr, "sensitive")
	assert.NotContains(t, nstr, "regular")

}

func Test_VariableType_Json_Sensitive(t *testing.T) {

	str := []byte("{\"name\": \"MyVar\", \"type\":\"sensitive\"}")

	input := Input{}

	err := json.Unmarshal(str, &input)

	assert.NoError(t, err)

	assert.False(t, input.Type.IsRegular())
	assert.False(t, input.Type.IsLocal())
	assert.True(t, input.Type.IsSensitive())

	nbytes, err := json.Marshal(input)
	nstr := string(nbytes)

	assert.NoError(t, err)

	assert.NotContains(t, nstr, "local")
	assert.Contains(t, nstr, "sensitive")
	assert.NotContains(t, nstr, "regular")

}

func Test_VariableType_Json_Both(t *testing.T) {

	str := []byte("{\"name\": \"MyVar\", \"type\":\"local-sensitive\"}")

	input := Input{}

	err := json.Unmarshal(str, &input)

	assert.NoError(t, err)

	assert.False(t, input.Type.IsRegular())
	assert.True(t, input.Type.IsLocal())
	assert.True(t, input.Type.IsSensitive())

	nbytes, err := json.Marshal(input)
	nstr := string(nbytes)

	assert.NoError(t, err)

	assert.Contains(t, nstr, "local-sensitive")
	assert.NotContains(t, nstr, "regular")

}

func Test_VariableType_Json_ParseError(t *testing.T) {

	str := []byte("{\"name\": \"MyVar\", \"type\":\"invalidtype\"}")

	input := Input{}

	err := json.Unmarshal(str, &input)

	assert.Error(t, err)

}

func Test_AnchoredRegex_Parse(t *testing.T) {
	pat := []byte("\"Then I froober the bazzer\"")

	var match AnchoredRegexp

	err := json.Unmarshal(pat, &match)

	assert.NoError(t, err)

	m := match.FindStringSubmatch("Then I froober the bazzer")
	assert.True(t, len(m) > 0)

	m2 := match.FindStringSubmatch("Then I froober the bazzer in the quxxer")
	assert.False(t, len(m2) > 0)
}

func Test_AnchoredRegex_Parse_PartiallyAnchored(t *testing.T) {
	pat := []byte("\"^Then I froober the bazzer\"")

	var match AnchoredRegexp

	err := json.Unmarshal(pat, &match)

	assert.NoError(t, err)

	m := match.FindStringSubmatch("Then I froober the bazzer")
	assert.True(t, len(m) > 0)

	m2 := match.FindStringSubmatch("Then I froober the bazzer in the quxxer")
	assert.True(t, len(m2) > 0)
}

func Test_AnchoredRegex_ParseError(t *testing.T) {
	pat := []byte("\"Then I froober the bazzer")

	var match AnchoredRegexp

	err := json.Unmarshal(pat, &match)

	assert.Error(t, err)
}

func Test_AnchoredRegex_Marshal(t *testing.T) {
	r, err := regexp.Compile("^foo$")
	require.NoError(t, err)

	a := AnchoredRegexp{
		Regexp:   *r,
		anchored: true,
	}

	pat, err := json.Marshal(a)

	assert.NoError(t, err)
	assert.Equal(t, []byte("\"foo\""), pat)
}

func Test_AnchoredRegex_Marshal_LiteralAnchors(t *testing.T) {
	r, err := regexp.Compile("^foo$")
	require.NoError(t, err)

	a := AnchoredRegexp{
		Regexp:   *r,
		anchored: false,
	}

	pat, err := json.Marshal(a)

	assert.NoError(t, err)
	assert.Equal(t, []byte("\"^foo$\""), pat)
}
