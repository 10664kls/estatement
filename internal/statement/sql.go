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
	IsSent  *bool   `json:"isSent"`
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
	CreatedBefore time.Time `json:"createdBefore"`
	CreatedAfter  time.Time `json:"createdAfter"`
	Gender        string    `json:"gender"`
	Status        string    `json:"status"`
	QueueNumber   string    `json:"queueNumber"`
	ProductName   string    `json:"productName"`
	BankCode      string    `json:"bankCode"`
	CreatedBy     string    `json:"createdBy"`
	Term          uint64    `json:"term"`
	PageToken     string    `json:"pageToken"`
	PageSize      uint64    `json:"pageSize"`
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
	if q.Term != 0 {
		and = append(and, sq.Eq{"term": q.Term})
	}
	if q.CreatedBy != "" {
		and = append(and, sq.Eq{"createdby": q.CreatedBy})
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
			"createdate",
			"bankstatus",
			"bankmoreinfo",
			"gender",
			"productnames",
			"emailstatus",
			"emailmsg",
			"occupation",
			"createby",
			"statusBanking",
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
			&s.Email.IsSent,
			&s.Email.Message,
			&s.Customer.Occupation,
			&s.CreatedBy,
			&s.Status,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStatementNotFound
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		statements = append(statements, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return statements, nil
}
