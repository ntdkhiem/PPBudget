package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"net/smtp"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

type SimplefinClaimRequest struct {
	SetupToken string `json:"setup_token"`
}

type SimplefinClaimResponse struct {
	AccessURL string `json:"access_url"`
}

type SimplefinFetchAccountsRequest struct {
	AccessURL string `json:"access_url"`
}

type SimplefinFetchAccountsResponse struct {
	SFAccounts []SFAccount      `json:"sf_accounts"`
	PPAccounts []domain.Account `json:"pp_accounts"`
}

type SFAccount struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Currency     string          `json:"currency"`
	Balance      string          `json:"balance"`
	Transactions []sfTransaction `json:"transactions,omitempty"`
}

type sfTransaction struct {
	ID          string `json:"id"`
	Posted      int64  `json:"posted"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	Pending     bool   `json:"pending"`
}

type sfResponse struct {
	Accounts []SFAccount `json:"accounts"`
}

type SimplefinExecuteRequest struct {
	AccessURL      string            `json:"access_url"`
	AccountMapping map[string]string `json:"account_mapping"`
	StartDate      string            `json:"start_date"` // YYYY-MM-DD
	ImportPending  bool              `json:"import_pending"`
	ApplyRules     bool              `json:"apply_rules"`
	ContentDedup   bool              `json:"content_dedup"`
	IsAutoSync     bool              `json:"-"`
}

// 1. Claim
func (s *Service) SimpleFinClaim(ctx context.Context, req SimplefinClaimRequest) (*SimplefinClaimResponse, error) {
	token := req.SetupToken

	// Check if it's a raw access URL
	if strings.HasPrefix(token, "https://") && strings.Contains(token, "@") && strings.Contains(token, "simplefin.org") {
		return &SimplefinClaimResponse{AccessURL: token}, nil
	}

	// Check if it's a simplefin.json configuration
	var config struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal([]byte(token), &config); err == nil && config.AccessToken != "" {
		return &SimplefinClaimResponse{AccessURL: config.AccessToken}, nil
	}

	// Assume it's a base64 encoded claim URL
	claimURLBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		claimURLBytes = []byte(token) // maybe it's not base64
	}
	claimURL := string(claimURLBytes)

	if !strings.HasPrefix(claimURL, "https://") {
		return nil, fmt.Errorf("invalid setup token format")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, claimURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create claim request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute claim request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claim request failed with status: %d", resp.StatusCode)
	}

	accessURLBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read access url: %w", err)
	}

	accessURL := string(accessURLBytes)

	// Save to simplefin.json
	configData := map[string]interface{}{
		"access_token": accessURL,
		"flow":         "simplefin",
	}
	if b, err := json.MarshalIndent(configData, "", "  "); err == nil {
		_ = os.WriteFile("simplefin.json", b, 0644)
	}

	return &SimplefinClaimResponse{AccessURL: accessURL}, nil
}

// 2. Fetch Accounts
func (s *Service) SimpleFinFetchAccounts(ctx context.Context, req SimplefinFetchAccountsRequest) (*SimplefinFetchAccountsResponse, error) {
	accountsURL := req.AccessURL + "/accounts"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, accountsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts request: %w", err)
	}
	if httpReq.URL.User != nil {
		password, _ := httpReq.URL.User.Password()
		httpReq.SetBasicAuth(httpReq.URL.User.Username(), password)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("accounts request failed with status: %d", resp.StatusCode)
	}

	var sfData sfResponse
	if err := json.NewDecoder(resp.Body).Decode(&sfData); err != nil {
		return nil, fmt.Errorf("failed to parse simplefin response: %w", err)
	}

	ppAccounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ppbudget accounts: %w", err)
	}

	// Remove transactions to save payload size for the frontend
	for i := range sfData.Accounts {
		sfData.Accounts[i].Transactions = nil
	}

	return &SimplefinFetchAccountsResponse{
		SFAccounts: sfData.Accounts,
		PPAccounts: ppAccounts,
	}, nil
}

var ImportProgress = struct {
	sync.RWMutex
	Status  string `json:"status"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Error   string `json:"error"`
}{Status: "idle"}

// 3. Execute
func (s *Service) SimpleFinExecute(ctx context.Context, req SimplefinExecuteRequest) error {
	accountsURL := req.AccessURL + "/accounts"

	// Format start date if provided
	var startTime time.Time
	if req.StartDate != "" {
		parsed, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			// If not YYYY-MM-DD, try RFC3339
			parsed, err = time.Parse(time.RFC3339, req.StartDate)
			if err != nil {
				s.logger.Error("failed to parse start date", "error", err, "start_date", req.StartDate)
			} else {
				startTime = parsed
			}
		} else {
			startTime = parsed
		}

		if !startTime.IsZero() {
			accountsURL += fmt.Sprintf("?start-date=%d", startTime.Unix())
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, accountsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create execute request: %w", err)
	}
	if httpReq.URL.User != nil {
		password, _ := httpReq.URL.User.Password()
		httpReq.SetBasicAuth(httpReq.URL.User.Username(), password)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to fetch accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("accounts request failed with status: %d", resp.StatusCode)
	}

	var sfData sfResponse
	if err := json.NewDecoder(resp.Body).Decode(&sfData); err != nil {
		return fmt.Errorf("failed to parse simplefin response: %w", err)
	}

	totalTransactions := 0
	for _, acc := range sfData.Accounts {
		mappedAccountID, ok := req.AccountMapping[acc.ID]
		if !ok || mappedAccountID == "" {
			continue // Skip unmapped accounts
		}
		
		for _, txn := range acc.Transactions {
			if txn.Pending && !req.ImportPending {
				continue
			}

			date := time.Unix(txn.Posted, 0)
			if !startTime.IsZero() && date.Before(startTime) {
				continue
			}
			totalTransactions++
		}
	}

	ImportProgress.Lock()
	ImportProgress.Status = "running"
	ImportProgress.Current = 0
	ImportProgress.Total = totalTransactions
	ImportProgress.Error = ""
	ImportProgress.Unlock()

	// Save account mapping to simplefin.json
	b, err := os.ReadFile("simplefin.json")
	if err == nil {
		var config map[string]interface{}
		if err := json.Unmarshal(b, &config); err == nil {
			config["account_mapping"] = req.AccountMapping
			config["import_pending"] = req.ImportPending
			config["apply_rules"] = req.ApplyRules
			config["content_dedup"] = req.ContentDedup
			if out, err := json.MarshalIndent(config, "", "  "); err == nil {
				_ = os.WriteFile("simplefin.json", out, 0644)
			}
		}
	}

	go func() {
		// Use a background context for the goroutine since the request context might be cancelled
		bgCtx := context.Background()
		var importedTxns []domain.Transaction

		for _, acc := range sfData.Accounts {
			mappedAccountID, ok := req.AccountMapping[acc.ID]
			if !ok || mappedAccountID == "" {
				continue // Skip unmapped accounts
			}

			var targetAccountID string
			if mappedAccountID == "new" {
				balance, err := money.NewFromString(acc.Balance)
				if err != nil {
					balance = money.Money(0)
				}
				newID, err := s.repo.UpsertSimplefinAccount(bgCtx, acc.ID, acc.Name, acc.Currency, balance.ToInt64())
				if err != nil {
					s.logger.Error("failed to create account", "error", err, "sf_id", acc.ID)
					continue
				}
				targetAccountID = newID
			} else {
				targetAccountID = mappedAccountID
				// Link the simplefin account to the PP account if it hasn't been linked yet.
				_ = s.repo.LinkSimplefinAccount(bgCtx, targetAccountID, acc.ID)
			}

			tx, err := s.repo.BeginTx(bgCtx)
			if err != nil {
				s.logger.Error("failed to begin tx", "error", err)
				continue
			}

			for _, txn := range acc.Transactions {
				if txn.Pending && !req.ImportPending {
					continue
				}

				date := time.Unix(txn.Posted, 0)
				if !startTime.IsZero() && date.Before(startTime) {
					continue
				}

				amount, err := money.NewFromString(txn.Amount)
				if err != nil {
					ImportProgress.Lock()
					ImportProgress.Current++
					ImportProgress.Unlock()
					continue
				}

				if req.ContentDedup {
					exists, err := s.repo.TransactionExistsByDetails(bgCtx, targetAccountID, amount.ToInt64(), date, txn.Description)
					if err == nil && exists {
						ImportProgress.Lock()
						ImportProgress.Current++
						ImportProgress.Unlock()
						continue // Duplicate found based on content, skip
					}
				}

				txnID, created, err := s.repo.InsertIngestedTransaction(bgCtx, tx, targetAccountID, amount, date, txn.Description, txn.ID, nil, nil, false)
				if err != nil {
					s.logger.Error("failed to insert transaction", "error", err, "sf_txn_id", txn.ID)
				} else if created {
					importedTxns = append(importedTxns, domain.Transaction{
						ID: txnID,
						AccountID: targetAccountID,
						Amount: amount,
						Date: date,
						Description: txn.Description,
					})
				}

				ImportProgress.Lock()
				ImportProgress.Current++
				ImportProgress.Unlock()
			}

			if err := tx.Commit(bgCtx); err != nil {
				s.logger.Error("failed to commit tx", "error", err)
			}
		}

		// Apply all active rules to imported transactions
		if req.ApplyRules {
			rules, err := s.ListRulesDetailed(bgCtx)
			if err == nil {
				var sDate *time.Time
				if !startTime.IsZero() {
					sDate = &startTime
				}
				for _, r := range rules {
					if !r.IsActive {
						continue
					}
					_, err := s.ApplyRule(bgCtx, r.ID, false, sDate, nil)
					if err != nil {
						s.logger.Error("failed to apply rule during import", "error", err, "rule_id", r.ID)
					}
				}
			}
		}

		s.sendImportNotification(bgCtx, importedTxns, req.IsAutoSync)

		ImportProgress.Lock()
		ImportProgress.Status = "completed"
		ImportProgress.Unlock()
	}()

	return nil
}

func (s *Service) sendImportNotification(ctx context.Context, txns []domain.Transaction, autoSync bool) {
	if len(txns) == 0 {
		return
	}
	if s.cfg.SMTPHost == "" || s.cfg.NotificationEmail == "" {
		s.logger.Info("smtp not configured, skipping email notification")
		return
	}

	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)

	// Fetch account names for nicer email
	accounts, _ := s.ListAccounts(ctx)
	accMap := make(map[string]string)
	for _, a := range accounts {
		accMap[a.ID] = a.Name
	}

	// Refresh txns from DB to pick up any category assignments from rules
	var updatedTxns []domain.Transaction
	for _, txn := range txns {
		t, err := s.GetTransaction(ctx, txn.ID)
		if err == nil {
			updatedTxns = append(updatedTxns, *t)
		} else {
			updatedTxns = append(updatedTxns, txn)
		}
	}
	txns = updatedTxns

	// Fetch categories for nicer email
	categories, _ := s.ListCategories(ctx)
	catMap := make(map[string]string)
	for _, c := range categories {
		catMap[c.ID] = c.Name
	}

	categorizedCount := 0
	uncategorizedCount := 0
	txnsByAccount := make(map[string][]domain.Transaction)

	for _, txn := range txns {
		if txn.CategoryID != nil && *txn.CategoryID != "" {
			categorizedCount++
		} else {
			uncategorizedCount++
		}
		txnsByAccount[txn.AccountID] = append(txnsByAccount[txn.AccountID], txn)
	}

	var body strings.Builder
	body.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: #333; max-width: 650px; margin: 0 auto; padding: 20px; }
  .header { background-color: #f8f9fa; padding: 24px; border-radius: 8px; text-align: center; margin-bottom: 24px; border: 1px solid #e9ecef; }
  .header h2 { margin: 0; color: #212529; font-size: 24px; }
  .header p { margin: 12px 0 0 0; color: #6c757d; font-size: 16px; }
  .txn-table { width: 100%; border-collapse: collapse; margin-bottom: 24px; font-size: 14px; }
  .txn-table th, .txn-table td { padding: 12px 8px; text-align: left; border-bottom: 1px solid #dee2e6; }
  .txn-table th { background-color: #f8f9fa; font-weight: 600; color: #495057; }
  .txn-table td { color: #212529; }
  .amount { font-weight: 600; text-align: right; }
  .amount.positive { color: #2b8a3e; }
  .amount.negative { color: #c92a2a; }
  .btn { display: inline-block; padding: 6px 12px; background-color: #e9ecef; color: #495057; text-decoration: none; border-radius: 6px; font-size: 13px; font-weight: 500; transition: background-color 0.2s; }
  .btn:hover { background-color: #dee2e6; }
  .btn-primary { background-color: #18181b; color: #ffffff; padding: 10px 20px; font-size: 15px; border-radius: 8px; }
  .btn-primary:hover { background-color: #27272a; }
  .footer { margin-top: 32px; padding-top: 16px; border-top: 1px solid #e9ecef; text-align: center; font-size: 13px; color: #adb5bd; }
</style>
</head>
<body>
`)

	body.WriteString("<div class=\"header\">")
	if autoSync {
		body.WriteString("<h2>Auto-Sync Complete</h2>")
	} else {
		body.WriteString("<h2>Import Complete</h2>")
	}
	body.WriteString(fmt.Sprintf("<p>Successfully imported <strong>%d</strong> new transactions.</p>", len(txns)))
	
	body.WriteString("<div style=\"margin-top: 16px; font-size: 14px;\">")
	body.WriteString(fmt.Sprintf("<span style=\"display: inline-block; margin: 0 8px; color: #2b8a3e; background-color: #ebfbee; padding: 4px 12px; border-radius: 12px; font-weight: 500;\">✓ %d Categorized</span>", categorizedCount))
	if uncategorizedCount > 0 {
		body.WriteString(fmt.Sprintf("<span style=\"display: inline-block; margin: 0 8px; color: #c92a2a; background-color: #fff5f5; padding: 4px 12px; border-radius: 12px; font-weight: 500;\">⚠ %d Need Review</span>", uncategorizedCount))
	}
	body.WriteString("</div>")
	body.WriteString("</div>")

	for accID, accTxns := range txnsByAccount {
		accName := accMap[accID]
		if accName == "" {
			accName = "Unknown"
		}
		
		body.WriteString(fmt.Sprintf("<h3 style=\"margin-top: 32px; color: #495057; border-bottom: 2px solid #e9ecef; padding-bottom: 8px;\">%s <span style=\"font-weight: normal; font-size: 14px; color: #868e96;\">(%d transactions)</span></h3>", accName, len(accTxns)))
		
		body.WriteString("<table class=\"txn-table\">")
		body.WriteString("<thead><tr><th>Date</th><th>Description</th><th>Category</th><th style=\"text-align: right;\">Amount</th><th style=\"text-align: center;\">Action</th></tr></thead>")
		body.WriteString("<tbody>")

		for _, txn := range accTxns {
			link := fmt.Sprintf("%s/transactions?edit=%s", s.cfg.FrontendURL, txn.ID)
			
			amountStr := txn.Amount.String()
			amountClass := ""
			if strings.HasPrefix(amountStr, "-") {
				amountClass = "negative"
			} else if txn.Amount.ToInt64() > 0 {
				amountClass = "positive"
			}

			catName := ""
			if txn.CategoryID != nil && *txn.CategoryID != "" {
				catName = catMap[*txn.CategoryID]
			}
			if catName == "" {
				catName = "<span style=\"color: #adb5bd; font-style: italic;\">Uncategorized</span>"
			}

			body.WriteString(fmt.Sprintf(
				"<tr><td>%s</td><td>%s</td><td>%s</td><td class=\"amount %s\">%s</td><td style=\"text-align: center;\"><a class=\"btn\" href=\"%s\">View</a></td></tr>",
				txn.Date.Format("Jan 02, 2006"),
				txn.Description,
				catName,
				amountClass,
				amountStr,
				link,
			))
		}
		body.WriteString("</tbody></table>")
	}

	allTxnsLink := fmt.Sprintf("%s/transactions", s.cfg.FrontendURL)
	body.WriteString(fmt.Sprintf("<div style=\"text-align: center;\"><a href=\"%s\" class=\"btn btn-primary\">View All Transactions</a></div>", allTxnsLink))

	body.WriteString("<div class=\"footer\">Sent by PPBudget</div>")
	body.WriteString("</body></html>")

	msg := []byte("To: " + s.cfg.NotificationEmail + "\r\n" +
		"Subject: PPBudget Import Completed\r\n" +
		"MIME-version: 1.0;\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\";\r\n\r\n" +
		body.String())

	err := smtp.SendMail(s.cfg.SMTPHost+":"+s.cfg.SMTPPort, auth, s.cfg.SMTPUser, []string{s.cfg.NotificationEmail}, msg)
	if err != nil {
		s.logger.Error("failed to send import notification email", "error", err)
	} else {
		s.logger.Info("import notification email sent", "count", len(txns))
	}
}

// 4. Auto-Sync Background Job
func (s *Service) RunAutoSync(ctx context.Context) error {
	b, err := os.ReadFile("simplefin.json")
	if err != nil {
		s.logger.Info("auto-sync: no simplefin.json found, skipping")
		return nil // Not set up yet
	}

	var config struct {
		AccessToken    string            `json:"access_token"`
		AccountMapping map[string]string `json:"account_mapping"`
		ImportPending  bool              `json:"import_pending"`
		ApplyRules     bool              `json:"apply_rules"`
		ContentDedup   bool              `json:"content_dedup"`
		AutoSync       bool              `json:"auto_sync"`
	}
	if err := json.Unmarshal(b, &config); err != nil {
		return fmt.Errorf("auto-sync: failed to unmarshal config: %w", err)
	}

	if !config.AutoSync {
		s.logger.Info("auto-sync: disabled in config")
		return nil
	}
	if config.AccessToken == "" {
		return fmt.Errorf("auto-sync: missing access token")
	}

	// Calculate start date: 30 days ago
	startDate := time.Now().Add(-30 * 24 * time.Hour).Format("2006-01-02")

	req := SimplefinExecuteRequest{
		AccessURL:      config.AccessToken,
		AccountMapping: config.AccountMapping,
		StartDate:      startDate,
		ImportPending:  config.ImportPending,
		ApplyRules:     config.ApplyRules,
		ContentDedup:   config.ContentDedup,
		IsAutoSync:     true,
	}

	err = s.SimpleFinExecute(ctx, req)
	if err != nil {
		s.logger.Error("auto-sync: failed to execute simplefin import", "error", err)
		return err
	}
	s.logger.Info("auto-sync: successfully started import for last 30 days")
	return nil
}
