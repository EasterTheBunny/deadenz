package parse

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/ciphermountain/deadenz/pkg/components"
)

func ItemsFromJSON(b []byte) ([]components.Item, error) {
	type jsonItem struct {
		Name      string                `json:"name"`
		Findable  bool                  `json:"findable"`
		Usability *components.Usability `json:"usability,omitempty"`
		Mutators  []json.RawMessage     `json:"mutators,omitempty"`
	}

	type typer struct {
		Type string `json:"type"`
	}

	var loaded []jsonItem
	if err := json.Unmarshal(b, &loaded); err != nil {
		return nil, err
	}

	items := make([]components.Item, len(loaded))

	for idx, item := range loaded {
		mutators := make([]components.MutatorFunc, len(item.Mutators))
		for idx, conf := range item.Mutators {
			var typed typer
			if err := json.Unmarshal(conf, &typed); err != nil {
				return nil, err
			}

			var (
				mutator func([]byte) (components.MutatorFunc, error)
				err     error
			)

			switch typed.Type {
			case "stats":
				mutator = asStatMutator
			case "backpack_limit":
				mutator = asBackpackLimitMutator
			default:
				return nil, errors.New("unrecognized mutator type")
			}

			if mutators[idx], err = mutator(conf); err != nil {
				return nil, err
			}
		}

		items[idx] = components.Item{
			Type:      components.ItemType(idx + 1),
			Name:      item.Name,
			Findable:  item.Findable,
			Usability: item.Usability,
			Mutators:  mutators,
		}
	}

	return items, nil
}

func asStatMutator(data []byte) (components.MutatorFunc, error) {
	type jsonStatMutator struct {
		StatName string `json:"stat_name"`
		Mutation string `json:"mutation"`
	}

	var statMut jsonStatMutator
	if err := json.Unmarshal(data, &statMut); err != nil {
		return nil, err
	}

	value, err := strconv.Atoi(statMut.Mutation)
	if err != nil {
		return nil, err
	}

	switch statMut.StatName {
	case "wit":
		return components.MutateWitBy(value), nil
	case "skill":
		return components.MutateSkillBy(value), nil
	case "humor":
		return components.MutateHumorBy(value), nil
	default:
		return nil, errors.New("invalid stat name")
	}
}

func asBackpackLimitMutator(data []byte) (components.MutatorFunc, error) {
	type mutator struct {
		Limit uint8 `json:"limit"`
	}

	var mut mutator
	if err := json.Unmarshal(data, &mut); err != nil {
		return nil, err
	}

	return components.BackpackLimitMutator(mut.Limit), nil
}
