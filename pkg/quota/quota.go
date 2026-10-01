package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"agy-tools/pkg/types"
)

// FormatResetCountdown formats a reset timestamp into "Resets in 2d 10h" style string
func FormatResetCountdown(resetTimeStr string) string {
	if resetTimeStr == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, resetTimeStr)
	if err != nil {
		return ""
	}

	diff := time.Until(t)
	if diff <= 0 {
		return "Ready to reset"
	}

	totalMins := int(diff.Minutes())
	days := totalMins / 1440
	hours := (totalMins % 1440) / 60
	mins := totalMins % 60

	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("Resets in %dd %dh", days, hours)
		}
		return fmt.Sprintf("Resets in %dd", days)
	}
	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("Resets in %dh %dm", hours, mins)
		}
		return fmt.Sprintf("Resets in %dh", hours)
	}
	if mins > 0 {
		return fmt.Sprintf("Resets in %dm", mins)
	}
	return "Resets in <1m"
}

// FetchQuota queries retrieveUserQuotaSummary with User-Agent: antigravity and builds QuotaData
func FetchQuota(accessToken string, projectID string) (*types.QuotaData, error) {
	endpoints := []string{
		"https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
	}

	reqBody := []byte("{}")
	if projectID != "" {
		reqBody, _ = json.Marshal(map[string]string{"project": projectID})
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, endpoint := range endpoints {
		req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "antigravity")
		req.Header.Set("X-Client-Name", "antigravity")
		req.Header.Set("X-Client-Version", "2.18.1")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		var apiResp struct {
			Groups []struct {
				DisplayName string `json:"displayName"`
				Description string `json:"description"`
				Buckets     []struct {
					BucketID          string  `json:"bucketId"`
					DisplayName       string  `json:"displayName"`
					Description       string  `json:"description"`
					Window            string  `json:"window"`
					RemainingFraction float64 `json:"remainingFraction"`
					Remaining         *struct {
						Value float64 `json:"value"`
					} `json:"remaining"`
					Disabled  bool `json:"disabled"`
					ResetTime any  `json:"resetTime"`
				} `json:"buckets"`
			} `json:"groups"`
		}

		if err := json.Unmarshal(body, &apiResp); err != nil {
			continue
		}

		if len(apiResp.Groups) == 0 {
			continue
		}

		var groups []types.QuotaGroup
		var models []types.ModelQuota

		for _, g := range apiResp.Groups {
			group := types.QuotaGroup{
				DisplayName: g.DisplayName,
				Description: g.Description,
			}
			for _, b := range g.Buckets {
				fraction := b.RemainingFraction
				if b.Remaining != nil && b.Remaining.Value > 0 {
					fraction = b.Remaining.Value
				}

				resetTimeStr := ""
				if b.ResetTime != nil {
					switch v := b.ResetTime.(type) {
					case string:
						resetTimeStr = v
					case map[string]interface{}:
						if secVal, ok := v["seconds"]; ok {
							switch sec := secVal.(type) {
							case float64:
								resetTimeStr = time.Unix(int64(sec), 0).Format(time.RFC3339)
							case string:
								s, _ := strconv.ParseInt(sec, 10, 64)
								resetTimeStr = time.Unix(s, 0).Format(time.RFC3339)
							}
						}
					}
				}

				pct := math.Round(fraction*1000) / 10
				countdown := FormatResetCountdown(resetTimeStr)

				bucket := types.QuotaBucket{
					BucketID:          b.BucketID,
					DisplayName:       b.DisplayName,
					Description:       b.Description,
					Window:            b.Window,
					RemainingFraction: fraction,
					Percentage:        pct,
					ResetTime:         resetTimeStr,
					Subtext:           countdown,
					Disabled:          b.Disabled,
				}
				group.Buckets = append(group.Buckets, bucket)

				models = append(models, types.ModelQuota{
					Name:       fmt.Sprintf("%s: %s", g.DisplayName, b.DisplayName),
					Percentage: pct,
					ResetTime:  resetTimeStr,
				})
			}
			groups = append(groups, group)
		}

		return &types.QuotaData{
			Models:      models,
			LastUpdated: time.Now().UnixMilli(),
			Groups:      groups,
		}, nil
	}

	return nil, fmt.Errorf("failed to fetch quota summary from any endpoint")
}
