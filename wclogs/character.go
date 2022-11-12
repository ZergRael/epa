package wclogs

import (
	"context"
	"errors"
	"github.com/machinebox/graphql"
)

// classIDCanHeal defines a collection of classes capable of healing
var classIDCanHeal = []int{2, 5, 6, 7, 9}

// Character represents character info
type Character struct {
	ID      int
	Name    string
	Server  string
	Region  string
	ClassID int
}

// Slug returns printable Character identifier
func (t *Character) Slug() string {
	return t.Name + " " + t.Region + "-" + t.Server
}

// CanHeal returns true if Character should also be tracked as a healer
func (t *Character) CanHeal() bool {
	for _, classID := range classIDCanHeal {
		if classID == t.ClassID {
			return true
		}
	}

	return false
}

// GetCharacter queries WarcraftLogs character info based on character name, server and server region
func (w *WCLogs) GetCharacter(name, server, region string) (*Character, error) {
	req := graphql.NewRequest(`
    query ($name: String!, $server: String!, $region: String!) {
		characterData {
			character(name: $name, serverSlug: $server, serverRegion: $region) {
				id
				name
				classID
				server {
					name
					region {
						slug
					}
				}
			}
		}
    }
`)

	req.Var("name", name)
	req.Var("server", server)
	req.Var("region", region)

	var resp struct {
		CharacterData struct {
			Character struct {
				ID      int
				Name    string
				ClassID int
				Server  struct {
					Name   string
					Region struct {
						Slug string
					}
				}
			}
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	if resp.CharacterData.Character.ID == 0 {
		return nil, errors.New("character not found")
	}

	return &Character{
		ID:      resp.CharacterData.Character.ID,
		Name:    resp.CharacterData.Character.Name,
		Server:  resp.CharacterData.Character.Server.Name,
		Region:  resp.CharacterData.Character.Server.Region.Slug,
		ClassID: resp.CharacterData.Character.ClassID,
	}, nil
}
