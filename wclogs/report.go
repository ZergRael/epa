package wclogs

import (
	"context"
	"errors"
	"github.com/machinebox/graphql"
)

// Report represents WarcraftLogs report metadata
type Report struct {
	Code    string
	EndTime float64
	ZoneID  ZoneID
	Size    RaidSize
}

// GetLatestReportMetadata queries latest Report for a specific Character
func (w *WCLogs) GetLatestReportMetadata(char *Character) (*Report, error) {
	req := graphql.NewRequest(`
    query ($id: Int!) {
		characterData {
			character(id: $id) {
				recentReports(limit: 1) {
					data {
						code
						endTime
						zone {
							id
						}
						fights(killType: Kills) {
							encounterID
							size
						}
					}
				}
			}
		}
    }
`)

	req.Var("id", char.ID)

	var resp struct {
		CharacterData struct {
			Character struct {
				RecentReports struct {
					Data []struct {
						Code    string
						EndTime float64
						Zone    struct {
							ID int
						}
						// TODO: Get fights only when needed (EndTime diff)
						Fights []struct {
							EncounterID int
							Size        int
						}
					}
				}
			}
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	if len(resp.CharacterData.Character.RecentReports.Data) < 1 {
		return nil, errors.New("no recent report")
	}

	report := &resp.CharacterData.Character.RecentReports.Data[0]
	lastFight := report.Fights[len(report.Fights)-1]

	return &Report{
		Code:    report.Code,
		EndTime: report.EndTime,
		Size:    RaidSize(lastFight.Size),
		ZoneID:  cachedZones.GetZoneIDForEncounter(lastFight.EncounterID),
	}, nil
}
