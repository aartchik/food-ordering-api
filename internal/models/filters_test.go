package models

import (
	"testing"

	"food-ordering-api/internal/validator"
)

func TestValidateFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filters Filters
		valid   bool
	}{
		{
			name: "valid filters",
			filters: Filters{
				Page:         1,
				PageSize:     20,
				Sort:         "name",
				SortSafelist: []string{"name", "-name"},
			},
			valid: true,
		},
		{
			name: "rejects zero page",
			filters: Filters{
				Page:         0,
				PageSize:     20,
				Sort:         "name",
				SortSafelist: []string{"name"},
			},
			valid: false,
		},
		{
			name: "rejects large page size",
			filters: Filters{
				Page:         1,
				PageSize:     101,
				Sort:         "name",
				SortSafelist: []string{"name"},
			},
			valid: false,
		},
		{
			name: "rejects unsafe sort",
			filters: Filters{
				Page:         1,
				PageSize:     20,
				Sort:         "drop table restaurants",
				SortSafelist: []string{"name"},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := validator.New()
			ValidateFilters(v, tt.filters)

			if v.Valid() != tt.valid {
				t.Fatalf("valid = %v, want %v", v.Valid(), tt.valid)
			}
		})
	}
}

func TestCalculateMetadata(t *testing.T) {
	t.Parallel()

	meta := CalculateMetadata(55, 2, 20)

	if meta.CurrentPage != 2 {
		t.Fatalf("current page = %d, want 2", meta.CurrentPage)
	}
	if meta.LastPage != 3 {
		t.Fatalf("last page = %d, want 3", meta.LastPage)
	}
	if meta.TotalRecords != 55 {
		t.Fatalf("total records = %d, want 55", meta.TotalRecords)
	}
}

func TestCalculateMetadataWithEmptyResult(t *testing.T) {
	t.Parallel()

	meta := CalculateMetadata(0, 1, 20)

	if meta != (Metadata{}) {
		t.Fatalf("metadata = %+v, want zero value", meta)
	}
}

func TestFiltersSQLHelpers(t *testing.T) {
	t.Parallel()

	filters := Filters{
		Page:         3,
		PageSize:     25,
		Sort:         "-name",
		SortSafelist: []string{"name", "-name"},
	}

	if filters.Limit() != 25 {
		t.Fatalf("limit = %d, want 25", filters.Limit())
	}
	if filters.Offset() != 50 {
		t.Fatalf("offset = %d, want 50", filters.Offset())
	}
	if filters.SortColumn() != "name" {
		t.Fatalf("sort column = %q, want name", filters.SortColumn())
	}
	if filters.SortDirection() != "DESC" {
		t.Fatalf("sort direction = %q, want DESC", filters.SortDirection())
	}
}

func TestSortColumnPanicsForUnsafeSort(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	filters := Filters{
		Sort:         "created_at; drop table",
		SortSafelist: []string{"name"},
	}

	filters.SortColumn()
}
