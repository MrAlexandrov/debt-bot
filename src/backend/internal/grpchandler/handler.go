package grpchandler

import (
	"context"
	"time"

	pb "github.com/mralexandrov/debt-bot/backend/gen/debt/v1"
	"github.com/mralexandrov/debt-bot/backend/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserService describes the user-management operations required by this handler.
type UserService interface {
	Create(ctx context.Context, name string) (*domain.User, error)
	ResolveOrCreate(ctx context.Context, platform, externalID, name, username string) (*domain.User, bool, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, id, name string) (*domain.User, error)
}

// DealService describes the deal-management operations required by this handler.
type DealService interface {
	Create(ctx context.Context, title, createdBy string) (*domain.Deal, error)
	GetByID(ctx context.Context, id string) (*domain.Deal, error)
	ListByUserID(ctx context.Context, userID string) ([]*domain.Deal, error)
	AddParticipant(ctx context.Context, dealID, userID string) (*domain.Deal, error)
	SetCoverage(ctx context.Context, dealID, payerID, coveredID string) (*domain.Deal, error)
	RemoveCoverage(ctx context.Context, dealID, coveredID string) (*domain.Deal, error)
	AddPurchase(ctx context.Context, dealID, title string, amount int64, paidBy, splitMode string, participantIDs []string) (*domain.Purchase, error)
	ListPurchases(ctx context.Context, dealID string) ([]*domain.Purchase, error)
}

// DebtService describes the debt-calculation operations required by this handler.
type DebtService interface {
	Calculate(ctx context.Context, dealID string) (*domain.CalculationResult, error)
}

type Handler struct {
	pb.UnimplementedDebtServiceServer
	users UserService
	deals DealService
	debts DebtService
}

func New(users UserService, deals DealService, debts DebtService) *Handler {
	return &Handler{users: users, deals: deals, debts: debts}
}

// --- User ---

func (h *Handler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	user, err := h.users.Create(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create user: %v", err)
	}
	return &pb.CreateUserResponse{User: domainUserToProto(user)}, nil
}

func (h *Handler) ResolveOrCreateUser(ctx context.Context, req *pb.ResolveOrCreateUserRequest) (*pb.ResolveOrCreateUserResponse, error) {
	user, created, err := h.users.ResolveOrCreate(ctx, req.Platform, req.ExternalId, req.Name, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve or create user: %v", err)
	}
	return &pb.ResolveOrCreateUserResponse{
		User:    domainUserToProto(user),
		Created: created,
	}, nil
}

func (h *Handler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := h.users.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "get user: %v", err)
	}
	return &pb.GetUserResponse{User: domainUserToProto(user)}, nil
}

func (h *Handler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	user, err := h.users.Update(ctx, req.UserId, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update user: %v", err)
	}
	return &pb.UpdateUserResponse{User: domainUserToProto(user)}, nil
}

// --- Deal ---

func (h *Handler) CreateDeal(ctx context.Context, req *pb.CreateDealRequest) (*pb.CreateDealResponse, error) {
	deal, err := h.deals.Create(ctx, req.Title, req.CreatedBy)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create deal: %v", err)
	}
	return &pb.CreateDealResponse{Deal: domainDealToProto(deal)}, nil
}

func (h *Handler) GetDeal(ctx context.Context, req *pb.GetDealRequest) (*pb.GetDealResponse, error) {
	deal, err := h.deals.GetByID(ctx, req.DealId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "get deal: %v", err)
	}
	return &pb.GetDealResponse{Deal: domainDealToProto(deal)}, nil
}

func (h *Handler) ListUserDeals(ctx context.Context, req *pb.ListUserDealsRequest) (*pb.ListUserDealsResponse, error) {
	deals, err := h.deals.ListByUserID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list deals: %v", err)
	}
	var pbDeals []*pb.Deal
	for _, d := range deals {
		pbDeals = append(pbDeals, domainDealToProto(d))
	}
	return &pb.ListUserDealsResponse{Deals: pbDeals}, nil
}

func (h *Handler) AddDealParticipant(ctx context.Context, req *pb.AddDealParticipantRequest) (*pb.AddDealParticipantResponse, error) {
	deal, err := h.deals.AddParticipant(ctx, req.DealId, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add participant: %v", err)
	}
	return &pb.AddDealParticipantResponse{Deal: domainDealToProto(deal)}, nil
}

func (h *Handler) SetDealCoverage(ctx context.Context, req *pb.SetDealCoverageRequest) (*pb.SetDealCoverageResponse, error) {
	deal, err := h.deals.SetCoverage(ctx, req.DealId, req.PayerId, req.CoveredId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set deal coverage: %v", err)
	}
	return &pb.SetDealCoverageResponse{Deal: domainDealToProto(deal)}, nil
}

func (h *Handler) RemoveDealCoverage(ctx context.Context, req *pb.RemoveDealCoverageRequest) (*pb.RemoveDealCoverageResponse, error) {
	deal, err := h.deals.RemoveCoverage(ctx, req.DealId, req.CoveredId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "remove deal coverage: %v", err)
	}
	return &pb.RemoveDealCoverageResponse{Deal: domainDealToProto(deal)}, nil
}

// --- Purchase ---

func (h *Handler) AddPurchase(ctx context.Context, req *pb.AddPurchaseRequest) (*pb.AddPurchaseResponse, error) {
	purchase, err := h.deals.AddPurchase(ctx, req.DealId, req.Title, req.Amount, req.PaidBy, req.SplitMode, req.ParticipantIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add purchase: %v", err)
	}
	return &pb.AddPurchaseResponse{Purchase: domainPurchaseToProto(purchase)}, nil
}

func (h *Handler) ListDealPurchases(ctx context.Context, req *pb.ListDealPurchasesRequest) (*pb.ListDealPurchasesResponse, error) {
	purchases, err := h.deals.ListPurchases(ctx, req.DealId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list purchases: %v", err)
	}
	var pbPurchases []*pb.Purchase
	for _, p := range purchases {
		pbPurchases = append(pbPurchases, domainPurchaseToProto(p))
	}
	return &pb.ListDealPurchasesResponse{Purchases: pbPurchases}, nil
}

// --- Debt ---

func (h *Handler) CalculateDebts(ctx context.Context, req *pb.CalculateDebtsRequest) (*pb.CalculateDebtsResponse, error) {
	result, err := h.debts.Calculate(ctx, req.DealId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "calculate debts: %v", err)
	}

	var pbDebts []*pb.DebtItem
	for _, d := range result.Debts {
		pbDebts = append(pbDebts, &pb.DebtItem{
			FromUserId: d.FromUserID,
			ToUserId:   d.ToUserID,
			Amount:     d.Amount,
		})
	}

	return &pb.CalculateDebtsResponse{
		Debts:    pbDebts,
		Balances: result.Balances,
	}, nil
}

// --- Converters ---

func domainUserToProto(u *domain.User) *pb.User {
	return &pb.User{
		Id:        u.ID,
		Name:      u.Name,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

func domainDealToProto(d *domain.Deal) *pb.Deal {
	pbCoverages := make([]*pb.Coverage, len(d.Coverages))
	for i, c := range d.Coverages {
		pbCoverages[i] = &pb.Coverage{PayerId: c.PayerID, CoveredId: c.CoveredID}
	}
	return &pb.Deal{
		Id:             d.ID,
		Title:          d.Title,
		CreatedBy:      d.CreatedBy,
		CreatedAt:      d.CreatedAt.Format(time.RFC3339),
		ParticipantIds: d.ParticipantIDs,
		Coverages:      pbCoverages,
	}
}

func domainPurchaseToProto(p *domain.Purchase) *pb.Purchase {
	return &pb.Purchase{
		Id:             p.ID,
		DealId:         p.DealID,
		Title:          p.Title,
		Amount:         p.Amount,
		PaidBy:         p.PaidBy,
		SplitMode:      p.SplitMode,
		ParticipantIds: p.ParticipantIDs,
	}
}
