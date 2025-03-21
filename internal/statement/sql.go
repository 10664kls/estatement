package statement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/10664kls/estatement/internal/pager"
	sq "github.com/Masterminds/squirrel"
)

type Statement struct {
	ID          string      `json:"id"`
	QueueNumber string      `json:"queueNumber"`
	ProductName string      `json:"productName"`
	Customer    Customer    `json:"customer"`
	BankAccount BankAccount `json:"bankAccount"`
	Email       Email       `json:"email"`
	Status      string      `json:"status"`
	CreatedBy   string      `json:"createdBy"`
	CreatedAt   time.Time   `json:"createdAt"`
}

type Email struct {
	IsSent  *string `json:"isSent"`
	Message *string `json:"message"`
}

type Customer struct {
	Gender      string `json:"gender"`
	DisplayName string `json:"displayName"`
	Occupation  string `json:"occupation"`
}

type BankAccount struct {
	Number    string     `json:"number"`
	Term      string     `json:"term"`
	Code      string     `json:"code"`
	Status    *string    `json:"status"`
	Info      *string    `json:"info"`
	CreatedAt *time.Time `json:"createdAt"`
}

type ListStatementsResult struct {
	Statements    []*Statement `json:"statements"`
	NextPageToken string       `json:"nextPageToken"`
}

type StatementQuery struct {
	CreatedBefore time.Time `json:"createdBefore" query:"createdBefore"`
	CreatedAfter  time.Time `json:"createdAfter" query:"createdAfter"`
	Gender        string    `json:"gender" query:"gender"`
	Status        string    `json:"status" query:"status"`
	Occupation    string    `json:"occupation" query:"occupation"`
	QueueNumber   string    `json:"queueNumber" query:"queueNumber"`
	ProductName   string    `json:"productName" query:"productName"`
	BankCode      string    `json:"bankCode" query:"bankCode"`
	CreatedBy     string    `json:"createdBy" query:"createdBy"`
	Term          string    `json:"term" query:"term"`
	PageToken     string    `json:"pageToken" query:"pageToken"`
	PageSize      uint64    `json:"pageSize" query:"pageSize"`
}

func (q *StatementQuery) ToSql() (string, []any, error) {
	and := sq.And{}
	if q.Gender != "" {
		and = append(and, sq.Eq{"gender": q.Gender})
	}
	if q.Status != "" {
		and = append(and, sq.Eq{"statusBanking": q.Status})
	}
	if q.ProductName != "" {
		and = append(and, sq.Eq{"productnames": q.ProductName})
	}
	if q.BankCode != "" {
		and = append(and, sq.Eq{"bankname": q.BankCode})
	}
	if q.QueueNumber != "" {
		and = append(and, sq.Eq{"cusnum": q.QueueNumber})
	}
	if q.Term != "" {
		and = append(and, sq.Eq{"term": q.Term})
	}
	if q.CreatedBy != "" {
		and = append(and, sq.Eq{"createby": q.CreatedBy})
	}
	if q.Occupation != "" {
		and = append(and, sq.Eq{"occupation": q.Occupation})
	}

	if !q.CreatedBefore.IsZero() {
		and = append(and, sq.LtOrEq{"createdate": q.CreatedBefore})
	}
	if !q.CreatedAfter.IsZero() {
		and = append(and, sq.GtOrEq{"createdate": q.CreatedAfter})
	}

	if q.PageToken != "" {
		cursor, err := pager.DecodeCursor(q.PageToken)
		if err != nil {
			return "", nil, err
		}
		and = append(and, sq.Expr("CUID < ?", cursor.ID))
	}

	return and.ToSql()
}

func getStatements(ctx context.Context, db *sql.DB, in *StatementQuery) (*Statement, error) {
	statements, err := listStatements(ctx, db, in)
	if err != nil {
		return nil, err
	}
	if len(statements) == 0 {
		return nil, ErrStatementNotFound
	}

	return statements[0], nil
}

func listStatements(ctx context.Context, db *sql.DB, in *StatementQuery) ([]*Statement, error) {
	id := fmt.Sprintf("TOP %d CUID", pager.Size(in.PageSize))
	pred, args, err := in.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to convert to sql: %w", err)
	}

	q, args := sq.
		Select(
			id,
			"cusnum",
			"cus_name",
			"AccNo",
			"term",
			"bankname",
			"bankcreatedate",
			"bankstatus",
			"bankmoreinfo",
			"gender",
			"productnames",
			"emailstatus",
			"emailmsg",
			"occupation",
			"createby",
			"statusBanking",
			"createdate",
		).
		From("dbo.vm_customer").
		PlaceholderFormat(sq.AtP).
		Where(pred, args...).
		OrderBy("CUID DESC").
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	statements := make([]*Statement, 0)
	for rows.Next() {
		var s Statement
		var isSent sql.NullString
		err := rows.Scan(
			&s.ID,
			&s.QueueNumber,
			&s.Customer.DisplayName,
			&s.BankAccount.Number,
			&s.BankAccount.Term,
			&s.BankAccount.Code,
			&s.BankAccount.CreatedAt,
			&s.BankAccount.Status,
			&s.BankAccount.Info,
			&s.Customer.Gender,
			&s.ProductName,
			&isSent,
			&s.Email.Message,
			&s.Customer.Occupation,
			&s.CreatedBy,
			&s.Status,
			&s.CreatedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStatementNotFound
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if isSent.Valid {
			s.Email.IsSent = &isSent.String
		}

		statements = append(statements, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return statements, nil
}

func listProductNames(ctx context.Context, db *sql.DB) ([]string, error) {
	q, args := sq.
		Select("productnames").
		From("dbo.vm_customer").
		PlaceholderFormat(sq.AtP).
		GroupBy("productnames").
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	productNames := make([]string, 0)
	for rows.Next() {
		var productName string
		err := rows.Scan(&productName)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		productNames = append(productNames, productName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return productNames, nil
}

func listOccupations(ctx context.Context, db *sql.DB) ([]string, error) {
	q, args := sq.
		Select("occupation").
		From("dbo.vm_customer").
		PlaceholderFormat(sq.AtP).
		GroupBy("occupation").
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	occupations := make([]string, 0)
	for rows.Next() {
		var occupation string
		err := rows.Scan(&occupation)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		occupations = append(occupations, occupation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return occupations, nil
}

func listTerms(ctx context.Context, db *sql.DB) ([]string, error) {
	q, args := sq.
		Select("term").
		From("dbo.vm_customer").
		PlaceholderFormat(sq.AtP).
		GroupBy("term").
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	terms := make([]string, 0)
	for rows.Next() {
		var term string
		err := rows.Scan(&term)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		terms = append(terms, term)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return terms, nil
}
