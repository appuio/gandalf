package spells_test

import (
	"encoding/json"
	"testing"

	"github.com/appuio/gandalf/pkg/spells"
	"github.com/stretchr/testify/assert"
)

func Test_VariableType_Json_Regular(t *testing.T) {
	for _, str := range []string{
		"{\"name\": \"MyVar\"}",
		"{\"name\": \"MyVar\", \"type\":\"regular\"}",
		"{\"name\": \"MyVar\", \"type\":\"\"}",
	} {
		input := spells.Input{}

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

	input := spells.Input{}

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

	input := spells.Input{}

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

	input := spells.Input{}

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

	input := spells.Input{}

	err := json.Unmarshal(str, &input)

	assert.Error(t, err)

}
