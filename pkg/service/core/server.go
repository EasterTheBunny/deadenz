package core

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/ciphermountain/deadenz/internal/util"
	deadenz "github.com/ciphermountain/deadenz/pkg"
	"github.com/ciphermountain/deadenz/pkg/components"
	"github.com/ciphermountain/deadenz/pkg/events"
	"github.com/ciphermountain/deadenz/pkg/middleware"
	"github.com/ciphermountain/deadenz/pkg/parse"
	proto "github.com/ciphermountain/deadenz/pkg/proto/core"
	"github.com/ciphermountain/deadenz/pkg/service/multiverse"
)

var _ proto.DeadenzServer = &Server{}

type Server struct {
	proto.UnimplementedDeadenzServer
	loader       *util.DataLoader
	preCommands  []deadenz.PreRunFunc
	postCommands []deadenz.PostRunFunc
}

func NewServer(client *multiverse.Client) *Server {
	loader := util.NewDataLoader()
	items := util.NewItemProviderFromLoader(loader)

	return &Server{
		loader: loader,
		preCommands: []deadenz.PreRunFunc{
			middleware.WalkLimiter(12, items),
			middleware.WalkStatBuilder(2, items), // TODO: stats builder needs to be configured to items that can mutate stats
		},
		postCommands: []deadenz.PostRunFunc{
			middleware.PublishEventsToMultiverse(client),
			middleware.DeathActiveItemMiddleware(1, items), // TODO: death recovery active item needs to be configurable
			middleware.WalkDeathEventMiddleware(),
		},
	}
}

func (s *Server) Run(ctx context.Context, req *proto.RunRequest) (*proto.RunResponse, error) {
	var command deadenz.CommandType

	switch req.Command.(type) {
	case *proto.RunRequest_Walk:
		command = deadenz.WalkCommandType
	case *proto.RunRequest_Spawnin:
		command = deadenz.SpawninCommandType
	default:
		return &proto.RunResponse{
			Response: &proto.Response{
				Status:  proto.Status_Failure,
				Message: "unrecognized command",
			},
			Profile: req.GetProfile(),
		}, nil
	}

	profile := protoToProfile(req.GetProfile())

	result, err := deadenz.RunActionCommand(command, &profile, s.loader, s.preCommands, s.postCommands)
	if err != nil {
		return &proto.RunResponse{
			Response: &proto.Response{
				Status:  proto.Status_Failure,
				Message: err.Error(),
			},
			Profile: req.GetProfile(),
		}, nil
	}

	return &proto.RunResponse{
		Response: &proto.Response{
			Status: proto.Status_OK,
		},
		Profile: profileToProto(result.Profile),
		Events:  eventsToSlice(result.Events),
	}, nil
}

func (s *Server) Load(_ context.Context, req *proto.LoadRequest) (*proto.Response, error) {
	var (
		key    reflect.Type
		parser util.Parser
	)

	switch req.GetType() {
	case proto.AssetType_ItemAsset:
		key = itemType
		parser = decodeItems
	case proto.AssetType_CharacterAsset:
		key = characterType
		parser = decodeCharacters
	case proto.AssetType_ItemDecisionAsset:
		key = decType
		parser = json.Unmarshal
	case proto.AssetType_ActionAsset:
		key = actionType
		parser = json.Unmarshal
	case proto.AssetType_EncounterAsset:
		key = encType
		parser = json.Unmarshal
	case proto.AssetType_LiveMutationAsset:
		key = liveType
		parser = json.Unmarshal
	case proto.AssetType_DieMutationAsset:
		key = dieType
		parser = json.Unmarshal
	default:
		return &proto.Response{
			Status:  proto.Status_Failure,
			Message: "unrecognized asset type",
		}, nil
	}

	loader, err := getLoaderType(req)
	if err != nil {
		return &proto.Response{
			Status:  proto.Status_Failure,
			Message: err.Error(),
		}, nil
	}

	if err := s.loader.SetLoader(key, loader, parser); err != nil {
		return &proto.Response{
			Status:  proto.Status_Failure,
			Message: err.Error(),
		}, nil
	}

	return &proto.Response{Status: proto.Status_OK}, nil
}

func (s *Server) Assets(ctx context.Context, req *proto.AssetRequest) (*proto.AssetResponse, error) {
	switch req.GetType() {
	case proto.AssetType_ItemAsset:
		var items []components.Item

		if err := s.loader.LoadCtx(ctx, &items); err != nil {
			resp := &proto.AssetResponse{
				Response: &proto.Response{
					Status:  proto.Status_Failure,
					Message: err.Error(),
				},
			}

			return resp, nil
		}

		resp := &proto.AssetResponse{
			Response: &proto.Response{
				Status: proto.Status_OK,
			},
			Asset: &proto.AssetResponse_Item{
				Item: &proto.ItemAssetResponse{
					Items: mutateListValues(items, itemToProto),
				},
			},
		}

		return resp, nil
	case proto.AssetType_CharacterAsset:
		var characters []components.Character

		if err := s.loader.LoadCtx(ctx, &characters); err != nil {
			resp := &proto.AssetResponse{
				Response: &proto.Response{
					Status:  proto.Status_Failure,
					Message: err.Error(),
				},
			}

			return resp, nil
		}

		resp := &proto.AssetResponse{
			Response: &proto.Response{
				Status: proto.Status_OK,
			},
			Asset: &proto.AssetResponse_Character{
				Character: &proto.CharacterAssetResponse{
					Characters: mutateListValues(characters, characterToProto),
				},
			},
		}

		return resp, nil
	default:
		resp := &proto.AssetResponse{
			Response: &proto.Response{
				Status:  proto.Status_Failure,
				Message: "asset type unavailable",
			},
		}

		return resp, nil
	}
}

var (
	itemType      = reflect.TypeOf([]components.Item{})
	characterType = reflect.TypeOf([]components.Character{})
	decType       = reflect.TypeOf([]events.ItemDecisionEvent{})
	actionType    = reflect.TypeOf([]events.ActionEvent{})
	encType       = reflect.TypeOf([]events.EncounterEvent{})
	liveType      = reflect.TypeOf([]events.LiveMutationEvent{})
	dieType       = reflect.TypeOf([]events.DieMutationEvent{})
)

func decodeItems(data []byte, val any) error {
	items, err := parse.ItemsFromJSON(data)
	if err != nil {
		return err
	}

	reflect.Indirect(reflect.ValueOf(val)).Set(reflect.ValueOf(items))

	return nil
}

func decodeCharacters(data []byte, val any) error {
	chars, err := parse.CharactersFromJSON(data)
	if err != nil {
		return err
	}

	reflect.Indirect(reflect.ValueOf(val)).Set(reflect.ValueOf(chars))

	return nil
}

func protoToProfile(profile *proto.Profile) components.Profile {
	return components.Profile{
		UUID:          profile.Uuid,
		XP:            uint(profile.Xp),
		Currency:      uint(profile.Currency),
		Active:        protoToCharacterNil(profile.Active),
		ActiveItem:    protoToActiveItem(profile.ActiveItem),
		BackpackLimit: uint8(profile.BackpackLimit),
		Backpack:      protoToBackpack(profile.Backpack),
		Stats:         protoToStats(profile.Stats),
		Limits:        protoToLimits(profile.Limits),
	}
}

func profileToProto(profile *components.Profile) *proto.Profile {
	return &proto.Profile{
		Uuid:          profile.UUID,
		Xp:            uint64(profile.XP),
		Currency:      uint64(profile.Currency),
		Active:        characterNilToProto(profile.Active),
		ActiveItem:    activeItemToProto(profile.ActiveItem),
		BackpackLimit: uint32(profile.BackpackLimit),
		Backpack:      backpackToProto(profile.Backpack),
		Stats:         statsToProto(profile.Stats),
		Limits:        limitsToProto(profile.Limits),
	}
}

func protoToCharacterNil(char *proto.Character) *components.Character {
	if char == nil {
		return nil
	}

	character := protoToCharacter(char)

	return &character
}

func protoToCharacter(char *proto.Character) components.Character {
	if char == nil {
		return components.Character{}
	}

	return components.Character{
		Type:       components.CharacterType(char.Type),
		Name:       char.Name,
		Multiplier: uint8(char.Multiplier),
	}
}

func characterNilToProto(char *components.Character) *proto.Character {
	if char == nil {
		return nil
	}

	return characterToProto(*char)
}

func characterToProto(char components.Character) *proto.Character {
	return &proto.Character{
		Type:       uint64(char.Type),
		Name:       char.Name,
		Multiplier: uint32(char.Multiplier),
	}
}

func protoToActiveItem(item *uint64) *components.ItemType {
	if item == nil {
		return nil
	}

	val := components.ItemType(*item)

	return &val
}

func activeItemToProto(tp *components.ItemType) *uint64 {
	if tp == nil {
		return nil
	}

	val := uint64(*tp)

	return &val
}

func protoToBackpack(items []uint64) []components.ItemType {
	list := make([]components.ItemType, len(items))
	for idx, value := range items {
		list[idx] = components.ItemType(value)
	}

	return list
}

func backpackToProto(backpack []components.ItemType) []uint64 {
	slice := make([]uint64, len(backpack))
	for idx, value := range backpack {
		slice[idx] = uint64(value)
	}

	return slice
}

func protoToStats(stats *proto.Stats) components.Stats {
	if stats == nil {
		return components.Stats{}
	}

	return components.Stats{
		Wit:   int(stats.GetWit()),
		Skill: int(stats.GetSkill()),
		Humor: int(stats.GetHumor()),
	}
}

func protoToLimits(limits *proto.Limits) *components.Limits {
	if limits == nil {
		return nil
	}

	val, err := strconv.ParseUint(limits.WalkCount, 10, 64)
	if err != nil {
		panic(err)
	}

	return &components.Limits{
		LastWalk:  time.UnixMilli(limits.LastWalk),
		WalkCount: val,
	}
}

func limitsToProto(limits *components.Limits) *proto.Limits {
	if limits == nil {
		return nil
	}

	return &proto.Limits{
		LastWalk:  limits.LastWalk.UnixMilli(),
		WalkCount: strconv.FormatUint(limits.WalkCount, 10),
	}
}

func statsToProto(stats components.Stats) *proto.Stats {
	return &proto.Stats{
		Wit:   int32(stats.Wit),
		Skill: int32(stats.Skill),
		Humor: int32(stats.Humor),
	}
}

func eventsToSlice(events []components.Event) []string {
	slice := make([]string, len(events))
	for idx, event := range events {
		slice[idx] = event.String()
	}

	return slice
}

func getLoaderType(req *proto.LoadRequest) (util.Loader, error) {
	switch loader := req.Loader.(type) {
	case *proto.LoadRequest_FileLoader:
		return &FileLoader{Path: loader.FileLoader.GetPath()}, nil
	case *proto.LoadRequest_SqlLoader:
		return nil, fmt.Errorf("sql loader unsupported")
	default:
		return nil, fmt.Errorf("unknown data loader")
	}
}

func itemToProto(item components.Item) *proto.Item {
	return &proto.Item{
		Type: uint64(item.Type),
		Name: item.Name,
	}
}

func protoToItem(item *proto.Item) components.Item {
	return components.Item{
		Type: components.ItemType(item.Type),
		Name: item.Name,
	}
}

func mutateListValues[T any, P any](list []T, f func(T) P) []P {
	newList := make([]P, len(list))

	for i, val := range list {
		newList[i] = f(val)
	}

	return newList
}
