package cloud

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Billing-related errors.
var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrPaymentFailed        = errors.New("payment failed")
	ErrInvoiceNotFound      = errors.New("invoice not found")
)

// Subscription represents a billing subscription.
type Subscription struct {
	ID               string            `json:"id"`
	OrganizationID   string            `json:"organization_id"`
	Plan             Plan              `json:"plan"`
	Status           SubscriptionStatus `json:"status"`
	
	// Billing period
	CurrentPeriodStart time.Time       `json:"current_period_start"`
	CurrentPeriodEnd   time.Time       `json:"current_period_end"`
	
	// Payment details
	StripeSubscriptionID string        `json:"stripe_subscription_id,omitempty"`
	StripeCustomerID     string        `json:"stripe_customer_id,omitempty"`
	PaymentMethodID      string        `json:"payment_method_id,omitempty"`
	
	// Pricing
	BasePriceCents       int           `json:"base_price_cents"`
	DiscountPercent      int           `json:"discount_percent,omitempty"`
	
	// Timestamps
	TrialEndDate   *time.Time         `json:"trial_end_date,omitempty"`
	CanceledAt     *time.Time         `json:"canceled_at,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// SubscriptionStatus represents subscription state.
type SubscriptionStatus string

const (
	SubscriptionActive    SubscriptionStatus = "active"
	SubscriptionTrialing  SubscriptionStatus = "trialing"
	SubscriptionPastDue   SubscriptionStatus = "past_due"
	SubscriptionCanceled  SubscriptionStatus = "canceled"
	SubscriptionPaused    SubscriptionStatus = "paused"
)

// Invoice represents a billing invoice.
type Invoice struct {
	ID               string        `json:"id"`
	OrganizationID   string        `json:"organization_id"`
	SubscriptionID   string        `json:"subscription_id"`
	Number           string        `json:"number"`
	Status           InvoiceStatus `json:"status"`
	
	// Period
	PeriodStart      time.Time     `json:"period_start"`
	PeriodEnd        time.Time     `json:"period_end"`
	
	// Amounts (in cents)
	Subtotal         int           `json:"subtotal"`
	Tax              int           `json:"tax"`
	Total            int           `json:"total"`
	AmountPaid       int           `json:"amount_paid"`
	AmountDue        int           `json:"amount_due"`
	
	// Line items
	LineItems        []InvoiceLineItem `json:"line_items"`
	
	// Stripe integration
	StripeInvoiceID  string        `json:"stripe_invoice_id,omitempty"`
	StripePaymentURL string        `json:"stripe_payment_url,omitempty"`
	
	// Timestamps
	DueDate          time.Time     `json:"due_date"`
	PaidAt           *time.Time    `json:"paid_at,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
}

// InvoiceStatus represents invoice state.
type InvoiceStatus string

const (
	InvoiceDraft     InvoiceStatus = "draft"
	InvoiceOpen      InvoiceStatus = "open"
	InvoicePaid      InvoiceStatus = "paid"
	InvoiceVoid      InvoiceStatus = "void"
	InvoiceUncollectible InvoiceStatus = "uncollectible"
)

// InvoiceLineItem represents a line item on an invoice.
type InvoiceLineItem struct {
	Description  string `json:"description"`
	Quantity     int    `json:"quantity"`
	UnitPrice    int    `json:"unit_price_cents"`
	Amount       int    `json:"amount_cents"`
}

// UsageRecord represents metered usage for billing.
type UsageRecord struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	WorkspaceID    string    `json:"workspace_id"`
	Type           string    `json:"type"` // execution, storage, etc.
	Quantity       int64     `json:"quantity"`
	Timestamp      time.Time `json:"timestamp"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// BillingService manages subscriptions and billing.
type BillingService struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
	invoices      map[string]*Invoice
	usageRecords  []UsageRecord
	controlPlane  *ControlPlane
}

// NewBillingService creates a new billing service.
func NewBillingService(cp *ControlPlane) *BillingService {
	return &BillingService{
		subscriptions: make(map[string]*Subscription),
		invoices:      make(map[string]*Invoice),
		usageRecords:  make([]UsageRecord, 0),
		controlPlane:  cp,
	}
}

// CreateSubscription creates a new subscription for an organization.
func (bs *BillingService) CreateSubscription(ctx context.Context, orgID string, plan Plan) (*Subscription, error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	org, err := bs.controlPlane.GetOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}

	planDetails := bs.controlPlane.GetPlanDetails(plan)
	now := time.Now()

	sub := &Subscription{
		ID:                 uuid.New().String(),
		OrganizationID:     orgID,
		Plan:               plan,
		Status:             SubscriptionActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		BasePriceCents:     planDetails.BasePriceCents,
		StripeCustomerID:   org.StripeCustomerID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Free plan starts directly active, paid plans can have trial
	if plan != PlanFree && plan != PlanEnterprise {
		sub.Status = SubscriptionTrialing
		trialEnd := now.AddDate(0, 0, 14) // 14-day trial
		sub.TrialEndDate = &trialEnd
	}

	bs.subscriptions[sub.ID] = sub
	return sub, nil
}

// GetSubscription retrieves a subscription by ID.
func (bs *BillingService) GetSubscription(ctx context.Context, id string) (*Subscription, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	sub, ok := bs.subscriptions[id]
	if !ok {
		return nil, ErrSubscriptionNotFound
	}
	return sub, nil
}

// GetSubscriptionByOrg retrieves an organization's active subscription.
func (bs *BillingService) GetSubscriptionByOrg(ctx context.Context, orgID string) (*Subscription, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	for _, sub := range bs.subscriptions {
		if sub.OrganizationID == orgID && sub.Status != SubscriptionCanceled {
			return sub, nil
		}
	}
	return nil, ErrSubscriptionNotFound
}

// UpdateSubscription updates a subscription.
func (bs *BillingService) UpdateSubscription(ctx context.Context, sub *Subscription) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if _, ok := bs.subscriptions[sub.ID]; !ok {
		return ErrSubscriptionNotFound
	}

	sub.UpdatedAt = time.Now()
	bs.subscriptions[sub.ID] = sub
	return nil
}

// CancelSubscription cancels a subscription.
func (bs *BillingService) CancelSubscription(ctx context.Context, id string, immediate bool) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	sub, ok := bs.subscriptions[id]
	if !ok {
		return ErrSubscriptionNotFound
	}

	now := time.Now()
	sub.CanceledAt = &now
	sub.UpdatedAt = now

	if immediate {
		sub.Status = SubscriptionCanceled
		// Downgrade org to free plan
		bs.controlPlane.ChangePlan(ctx, sub.OrganizationID, PlanFree)
	} else {
		// Cancel at end of period
		sub.Status = SubscriptionActive // Still active until period end
	}

	return nil
}

// ChangePlan upgrades or downgrades a subscription.
func (bs *BillingService) ChangePlan(ctx context.Context, subID string, newPlan Plan) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	sub, ok := bs.subscriptions[subID]
	if !ok {
		return ErrSubscriptionNotFound
	}

	planDetails := bs.controlPlane.GetPlanDetails(newPlan)
	
	// Prorate: in production, calculate credit/charge
	sub.Plan = newPlan
	sub.BasePriceCents = planDetails.BasePriceCents
	sub.UpdatedAt = time.Now()

	// Update organization plan
	return bs.controlPlane.ChangePlan(ctx, sub.OrganizationID, newPlan)
}

// GenerateInvoice generates an invoice for the current billing period.
func (bs *BillingService) GenerateInvoice(ctx context.Context, subID string) (*Invoice, error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	sub, ok := bs.subscriptions[subID]
	if !ok {
		return nil, ErrSubscriptionNotFound
	}

	// Calculate usage-based charges
	usageCharges := bs.calculateUsageCharges(sub.OrganizationID, sub.Plan, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)

	lineItems := []InvoiceLineItem{
		{
			Description: "Base Plan - " + string(sub.Plan),
			Quantity:    1,
			UnitPrice:   sub.BasePriceCents,
			Amount:      sub.BasePriceCents,
		},
	}
	lineItems = append(lineItems, usageCharges...)

	subtotal := 0
	for _, item := range lineItems {
		subtotal += item.Amount
	}

	// Apply discount
	if sub.DiscountPercent > 0 {
		discount := subtotal * sub.DiscountPercent / 100
		lineItems = append(lineItems, InvoiceLineItem{
			Description: "Discount",
			Quantity:    1,
			UnitPrice:   -discount,
			Amount:      -discount,
		})
		subtotal -= discount
	}

	tax := subtotal * 0 / 100 // No tax for now
	total := subtotal + tax

	invoice := &Invoice{
		ID:             uuid.New().String(),
		OrganizationID: sub.OrganizationID,
		SubscriptionID: sub.ID,
		Number:         generateInvoiceNumber(),
		Status:         InvoiceOpen,
		PeriodStart:    sub.CurrentPeriodStart,
		PeriodEnd:      sub.CurrentPeriodEnd,
		Subtotal:       subtotal,
		Tax:            tax,
		Total:          total,
		AmountPaid:     0,
		AmountDue:      total,
		LineItems:      lineItems,
		DueDate:        time.Now().AddDate(0, 0, 30),
		CreatedAt:      time.Now(),
	}

	bs.invoices[invoice.ID] = invoice
	return invoice, nil
}

func (bs *BillingService) calculateUsageCharges(orgID string, plan Plan, start, end time.Time) []InvoiceLineItem {
	items := make([]InvoiceLineItem, 0)
	
	// Count executions in period
	execCount := 0
	for _, record := range bs.usageRecords {
		if record.OrganizationID == orgID && 
		   record.Type == "execution" &&
		   record.Timestamp.After(start) && 
		   record.Timestamp.Before(end) {
			execCount += int(record.Quantity)
		}
	}

	// Get plan details for overage calculation
	planDetails := bs.controlPlane.GetPlanDetails(plan)
	if planDetails.MaxExecutionsPerDay > 0 {
		// Calculate days in period
		days := int(end.Sub(start).Hours() / 24)
		included := planDetails.MaxExecutionsPerDay * days
		overage := execCount - included
		if overage > 0 && planDetails.PricePerExecutionCents > 0 {
			items = append(items, InvoiceLineItem{
				Description: "Execution overage",
				Quantity:    overage,
				UnitPrice:   planDetails.PricePerExecutionCents,
				Amount:      overage * planDetails.PricePerExecutionCents,
			})
		}
	}

	return items
}

// GetInvoice retrieves an invoice by ID.
func (bs *BillingService) GetInvoice(ctx context.Context, id string) (*Invoice, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	invoice, ok := bs.invoices[id]
	if !ok {
		return nil, ErrInvoiceNotFound
	}
	return invoice, nil
}

// ListInvoices lists invoices for an organization.
func (bs *BillingService) ListInvoices(ctx context.Context, orgID string) ([]*Invoice, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	result := make([]*Invoice, 0)
	for _, invoice := range bs.invoices {
		if invoice.OrganizationID == orgID {
			result = append(result, invoice)
		}
	}
	return result, nil
}

// PayInvoice marks an invoice as paid.
func (bs *BillingService) PayInvoice(ctx context.Context, id string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	invoice, ok := bs.invoices[id]
	if !ok {
		return ErrInvoiceNotFound
	}

	now := time.Now()
	invoice.Status = InvoicePaid
	invoice.AmountPaid = invoice.Total
	invoice.AmountDue = 0
	invoice.PaidAt = &now

	return nil
}

// RecordUsage records a usage event for billing.
func (bs *BillingService) RecordUsage(ctx context.Context, record *UsageRecord) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	bs.usageRecords = append(bs.usageRecords, *record)
	return nil
}

// GetUsageSummary returns usage summary for an organization.
func (bs *BillingService) GetUsageSummary(ctx context.Context, orgID string, start, end time.Time) (map[string]int64, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	summary := make(map[string]int64)
	for _, record := range bs.usageRecords {
		if record.OrganizationID == orgID &&
		   record.Timestamp.After(start) &&
		   record.Timestamp.Before(end) {
			summary[record.Type] += record.Quantity
		}
	}
	return summary, nil
}

// Helper functions

var invoiceCounter int

func generateInvoiceNumber() string {
	invoiceCounter++
	return time.Now().Format("2006") + "-" + uuid.New().String()[:8]
}

// ProcessSubscriptionRenewal processes subscription renewal at period end.
func (bs *BillingService) ProcessSubscriptionRenewal(ctx context.Context) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	now := time.Now()
	for _, sub := range bs.subscriptions {
		if sub.Status == SubscriptionCanceled {
			continue
		}

		// Check if period has ended
		if now.After(sub.CurrentPeriodEnd) {
			// Check if scheduled for cancellation
			if sub.CanceledAt != nil {
				sub.Status = SubscriptionCanceled
				bs.controlPlane.ChangePlan(ctx, sub.OrganizationID, PlanFree)
				continue
			}

			// Generate invoice for past period
			invoice, err := bs.GenerateInvoice(ctx, sub.ID)
			if err != nil {
				continue
			}

			// In production, attempt to charge payment method
			// For now, just mark as open
			_ = invoice

			// Advance to next period
			sub.CurrentPeriodStart = sub.CurrentPeriodEnd
			sub.CurrentPeriodEnd = sub.CurrentPeriodEnd.AddDate(0, 1, 0)
			sub.UpdatedAt = now

			// Check trial expiration
			if sub.Status == SubscriptionTrialing && sub.TrialEndDate != nil {
				if now.After(*sub.TrialEndDate) {
					sub.Status = SubscriptionActive
				}
			}
		}
	}

	return nil
}
