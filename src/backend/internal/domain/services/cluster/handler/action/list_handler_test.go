package action

import (
	"context"
	"errors"
	"testing"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

// --- Mock ListService ---

type mockListService struct {
	listFunc func(page, pageSize int) ([]map[string]interface{}, int64, error)
}

func (m *mockListService) List(page, pageSize int) ([]map[string]interface{}, int64, error) {
	return m.listFunc(page, pageSize)
}

// --- Tests ---

func TestListHandler_ReturnsPaginatedResults(t *testing.T) {
	svc := &mockListService{
		listFunc: func(page, pageSize int) ([]map[string]interface{}, int64, error) {
			if page != 1 {
				t.Fatalf("expected page 1, got %d", page)
			}
			if pageSize != 10 {
				t.Fatalf("expected page_size 10, got %d", pageSize)
			}
			return []map[string]interface{}{
				{"id": 1, "name": "item1"},
				{"id": 2, "name": "item2"},
			}, 42, nil
		},
	}

	handler := NewListHandler(svc)

	req := clustertypes.ActionRequest{
		Action: "inbound.list",
		Payload: map[string]interface{}{
			"page":      float64(1),
			"page_size": float64(10),
		},
	}

	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}

	data, ok := resp.Data.(clustertypes.PaginationResponse)
	if !ok {
		t.Fatal("expected PaginationResponse data")
	}
	if data.Total != 42 {
		t.Fatalf("expected total 42, got %d", data.Total)
	}
	if data.Page != 1 {
		t.Fatalf("expected page 1, got %d", data.Page)
	}
	if data.PageSize != 10 {
		t.Fatalf("expected page_size 10, got %d", data.PageSize)
	}

	items, ok := data.Items.([]map[string]interface{})
	if !ok {
		t.Fatal("expected items to be []map[string]interface{}")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestListHandler_UsesDefaultPagination(t *testing.T) {
	svc := &mockListService{
		listFunc: func(page, pageSize int) ([]map[string]interface{}, int64, error) {
			if page != 1 {
				t.Fatalf("expected default page 1, got %d", page)
			}
			if pageSize != 10 {
				t.Fatalf("expected default page_size 10, got %d", pageSize)
			}
			return []map[string]interface{}{}, 0, nil
		},
	}

	handler := NewListHandler(svc)

	req := clustertypes.ActionRequest{
		Action:  "inbound.list",
		Payload: map[string]interface{}{},
	}

	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}

	data, ok := resp.Data.(clustertypes.PaginationResponse)
	if !ok {
		t.Fatal("expected PaginationResponse data")
	}
	if data.Page != 1 {
		t.Fatalf("expected default page 1, got %d", data.Page)
	}
	if data.PageSize != 10 {
		t.Fatalf("expected default page_size 10, got %d", data.PageSize)
	}
}

func TestListHandler_CustomPageSize(t *testing.T) {
	svc := &mockListService{
		listFunc: func(page, pageSize int) ([]map[string]interface{}, int64, error) {
			if page != 2 {
				t.Fatalf("expected page 2, got %d", page)
			}
			if pageSize != 5 {
				t.Fatalf("expected page_size 5, got %d", pageSize)
			}
			return []map[string]interface{}{
				{"id": 6},
				{"id": 7},
				{"id": 8},
			}, 15, nil
		},
	}

	handler := NewListHandler(svc)

	req := clustertypes.ActionRequest{
		Action: "inbound.list",
		Payload: map[string]interface{}{
			"page":      float64(2),
			"page_size": float64(5),
		},
	}

	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %q", resp.Status)
	}

	data, ok := resp.Data.(clustertypes.PaginationResponse)
	if !ok {
		t.Fatal("expected PaginationResponse data")
	}
	if data.Page != 2 {
		t.Fatalf("expected page 2, got %d", data.Page)
	}
	if data.PageSize != 5 {
		t.Fatalf("expected page_size 5, got %d", data.PageSize)
	}
	if data.Total != 15 {
		t.Fatalf("expected total 15, got %d", data.Total)
	}
}

func TestListHandler_ReturnsErrorOnServiceFailure(t *testing.T) {
	svc := &mockListService{
		listFunc: func(page, pageSize int) ([]map[string]interface{}, int64, error) {
			return nil, 0, errors.New("database error")
		},
	}

	handler := NewListHandler(svc)

	req := clustertypes.ActionRequest{
		Action: "inbound.list",
		Payload: map[string]interface{}{
			"page":      float64(1),
			"page_size": float64(10),
		},
	}

	_, err := handler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when service fails, got nil")
	}
}
