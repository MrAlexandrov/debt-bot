package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/mralexandrov/debt-bot/backend/internal/domain"
)

// --- Fake ---

type fakeUserRepo struct {
	users      map[string]*domain.User
	identities []*domain.UserIdentity
	nextID     int
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users: make(map[string]*domain.User),
	}
}

func (r *fakeUserRepo) nextUserID() string {
	r.nextID++
	return fmt.Sprintf("user-%d", r.nextID)
}

func (r *fakeUserRepo) Create(_ context.Context, name string) (*domain.User, error) {
	u := &domain.User{ID: r.nextUserID(), Name: name}
	r.users[u.ID] = u
	return u, nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	return u, nil
}

func (r *fakeUserRepo) Update(_ context.Context, id, name string) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	u.Name = name
	return u, nil
}

func (r *fakeUserRepo) FindIdentity(_ context.Context, platform, externalID string) (*domain.UserIdentity, error) {
	for _, id := range r.identities {
		if id.Platform == platform && id.ExternalID == externalID {
			return id, nil
		}
	}
	return nil, nil
}

func (r *fakeUserRepo) CreateIdentity(_ context.Context, userID, platform, externalID string) (*domain.UserIdentity, error) {
	id := &domain.UserIdentity{UserID: userID, Platform: platform, ExternalID: externalID}
	r.identities = append(r.identities, id)
	return id, nil
}

// --- Tests ---

func TestResolveOrCreate_FoundByPrimaryIdentity(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)
	ctx := t.Context()

	existing := &domain.User{ID: "user-existing", Name: "Alice"}
	repo.users["user-existing"] = existing
	repo.identities = append(repo.identities, &domain.UserIdentity{
		UserID:     "user-existing",
		Platform:   "telegram",
		ExternalID: "tg-123",
	})

	user, created, err := svc.ResolveOrCreate(ctx, "telegram", "tg-123", "Alice", "alice_tg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected created=false for existing user")
	}
	if user.ID != "user-existing" {
		t.Errorf("expected user-existing, got %s", user.ID)
	}
}

func TestResolveOrCreate_LinksExistingUsernameIdentity(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)
	ctx := t.Context()

	// User was previously added by @username (e.g. manually typed), but never via real Telegram ID
	existing := &domain.User{ID: "user-from-username", Name: "Bob"}
	repo.users["user-from-username"] = existing
	repo.identities = append(repo.identities, &domain.UserIdentity{
		UserID:     "user-from-username",
		Platform:   platformTelegramUsername,
		ExternalID: "bob_tg",
	})

	// Now Bob logs in for the first time with their real Telegram ID
	user, created, err := svc.ResolveOrCreate(ctx, "telegram", "tg-456", "Bob", "bob_tg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected created=false — user was merged, not created")
	}
	if user.ID != "user-from-username" {
		t.Errorf("expected user-from-username, got %s", user.ID)
	}

	// Telegram ID identity should now exist
	linked, _ := repo.FindIdentity(ctx, "telegram", "tg-456")
	if linked == nil || linked.UserID != "user-from-username" {
		t.Error("telegram identity was not linked to the existing user")
	}
}

func TestResolveOrCreate_CreatesNewUser(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)
	ctx := t.Context()

	user, created, err := svc.ResolveOrCreate(ctx, "telegram", "tg-789", "Carol", "carol_tg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created=true for brand-new user")
	}
	if user.Name != "Carol" {
		t.Errorf("expected name Carol, got %s", user.Name)
	}

	// Both telegram ID and telegram_username identities should be stored
	byID, _ := repo.FindIdentity(ctx, "telegram", "tg-789")
	if byID == nil {
		t.Error("telegram identity not created")
	}
	byUsername, _ := repo.FindIdentity(ctx, platformTelegramUsername, "carol_tg")
	if byUsername == nil {
		t.Error("telegram_username identity not created")
	}
}

func TestResolveOrCreate_NonTelegramPlatform_NoUsernameMerge(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)
	ctx := t.Context()

	// Pre-existing user with a username-style identity — should NOT be merged
	// for non-telegram platforms.
	existing := &domain.User{ID: "user-web", Name: "Dave"}
	repo.users["user-web"] = existing
	repo.identities = append(repo.identities, &domain.UserIdentity{
		UserID:     "user-web",
		Platform:   platformTelegramUsername,
		ExternalID: "dave",
	})

	user, created, err := svc.ResolveOrCreate(ctx, "web", "web-dave", "Dave", "dave")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected a new user — web platform should not merge telegram_username identities")
	}
	if user.ID == "user-web" {
		t.Error("should not have returned the existing telegram_username user")
	}
}

func TestUserService_Create(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)

	user, err := svc.Create(t.Context(), "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "Alice" || user.ID == "" {
		t.Errorf("unexpected user: %+v", user)
	}
}

func TestUserService_GetByID(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)
	repo.users["u1"] = &domain.User{ID: "u1", Name: "Bob"}

	user, err := svc.GetByID(t.Context(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "u1" {
		t.Errorf("expected u1, got %s", user.ID)
	}
}

func TestUserService_Update(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)
	ctx := t.Context()

	repo.users["u1"] = &domain.User{ID: "u1", Name: "Old Name"}

	updated, err := svc.Update(ctx, "u1", "New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected 'New Name', got %q", updated.Name)
	}
}
