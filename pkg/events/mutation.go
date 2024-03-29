package events

import (
	"encoding/json"
	"errors"

	"github.com/ciphermountain/deadenz/internal/util"
)

const DefaultDieRate = 30

func NewRandomMutationEvent(live []LiveMutationEvent, die []DieMutationEvent, diePercent int64) Event {
	if util.Random(0, 100) < diePercent {
		return die[util.Random(0, int64(len(die)-1))]
	}

	return live[util.Random(0, int64(len(live)-1))]
}

type DieMutationEvent struct {
	value string
}

func (e DieMutationEvent) String() string {
	return e.value
}

func (e DieMutationEvent) MarshalJSON() ([]byte, error) {
	formatted := jsonMutationEvent{
		Type:    string(EventTypeMutation),
		Message: e.value,
		IsDeath: true,
	}

	return json.Marshal(formatted)
}

func (e *DieMutationEvent) UnmarshalJSON(data []byte) error {
	var formatted jsonMutationEvent

	if err := json.Unmarshal(data, &formatted); err != nil {
		return err
	}

	if !formatted.IsDeath {
		return errors.New("not a death event")
	}

	*e = DieMutationEvent{
		value: formatted.Message,
	}

	return nil
}

func UnmarshalMutationEvent(data []byte) (Event, error) {
	type action struct {
		Message string `json:"message"`
		IsDeath bool   `json:"isDeath"`
	}

	var loaded action

	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, err
	}

	if loaded.IsDeath {
		return DieMutationEvent{value: loaded.Message}, nil
	}

	return LiveMutationEvent{value: loaded.Message}, nil
}

func LoadMutations(b []byte) ([]LiveMutationEvent, []DieMutationEvent, error) {
	type action struct {
		Message string `json:"message"`
		IsDeath bool   `json:"isDeath"`
	}

	var loaded []action

	if err := json.Unmarshal(b, &loaded); err != nil {
		return nil, nil, err
	}

	liveevts := []LiveMutationEvent{}
	dieEvts := []DieMutationEvent{}

	for _, l := range loaded {
		if !l.IsDeath {
			liveevts = append(liveevts, LiveMutationEvent{
				value: l.Message,
			})
		} else {
			dieEvts = append(dieEvts, DieMutationEvent{
				value: l.Message,
			})
		}
	}

	return liveevts, dieEvts, nil
}

type LiveMutationEvent struct {
	value string
}

func (e LiveMutationEvent) String() string {
	return e.value
}

func (e LiveMutationEvent) MarshalJSON() ([]byte, error) {
	formatted := jsonMutationEvent{
		Type:    string(EventTypeMutation),
		Message: e.value,
		IsDeath: false,
	}

	return json.Marshal(formatted)
}

func (e *LiveMutationEvent) UnmarshalJSON(data []byte) error {
	var formatted jsonMutationEvent

	if err := json.Unmarshal(data, &formatted); err != nil {
		return err
	}

	if formatted.IsDeath {
		return errors.New("not a live event")
	}

	*e = LiveMutationEvent{
		value: formatted.Message,
	}

	return nil
}

type jsonMutationEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	IsDeath bool   `json:"isDeath"`
}
