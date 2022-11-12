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
	Zone    struct {
		ID ZoneID
	}
}

// GetLatestReportMetadata queries latest Report for a specific Character
func (w *WCLogs) GetLatestReportMetadata(char *Character) (*Report, error) {
	req := graphql.NewRequest(`
    query ($id: Int!) {
		characterData {
			character(id: $id) {
				recentReports(limit: 1) {
					data {
						endTime
						code
						zone {
							id
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
					Data []Report
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

	return &resp.CharacterData.Character.RecentReports.Data[0], nil
}
