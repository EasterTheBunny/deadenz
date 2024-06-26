package util_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ciphermountain/deadenz/internal/util"
	"github.com/ciphermountain/deadenz/internal/util/mocks"
)

func TestDataLoader(t *testing.T) {
	dataLoader := util.NewDataLoader()
	loadedType := reflect.TypeOf([]string{})
	loader := new(mocks.MockLoader)

	dataLoader.SetLoader(loadedType, loader, json.Unmarshal)
	loader.EXPECT().Data(mock.Anything).Return(encoded, nil)

	var output []string

	err := dataLoader.LoadCtx(context.Background(), &output)

	require.NoError(t, err)
	assert.Equal(t, strVals, output)
}

var (
	strVals    = []string{"one", "two", "three"}
	encoded, _ = json.Marshal(strVals)
)
