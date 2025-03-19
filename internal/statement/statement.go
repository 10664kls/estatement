package statement

import (
	"context"
	"database/sql"
	"errors"

	"github.com/10664kls/estatement/internal/pager"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	rpcstatus "google.golang.org/grpc/status"
)

// ErrStatementNotFound is returned when the statement is not found.
var ErrStatementNotFound = errors.New("statement not found")

type Service struct {
	db   *sql.DB
	zlog *zap.Logger
}

func NewService(_ context.Context, db *sql.DB, zlog *zap.Logger) (*Service, error) {
	s := &Service{
		db:   db,
		zlog: zlog,
	}

	return s, nil
}

func (s *Service) ListStatements(ctx context.Context, in *StatementQuery) (*ListStatementsResult, error) {
	zlog := s.zlog.With(zap.Any("query", in))

	zlog.Info("starting to list statements")

	statements, err := listStatements(ctx, s.db, in)
	if err != nil {
		zlog.Error("failed to list statements", zap.Error(err))
		return nil, err
	}

	var pageToken string
	if l := len(statements); l > 0 && l == int(pager.Size(in.PageSize)) {
		last := statements[l-1]
		pageToken = pager.EncodeCursor(&pager.Cursor{
			ID:   last.ID,
			Time: last.CreatedAt,
		})
	}

	return &ListStatementsResult{
		Statements:    statements,
		NextPageToken: pageToken,
	}, nil
}

func (s *Service) GetStatementByID(ctx context.Context, id string) (*Statement, error) {
	zlog := s.zlog.With(zap.Any("id", id))

	zlog.Info("starting to get statement by id")

	statement, err := getStatements(ctx, s.db, &StatementQuery{QueueNumber: id})
	if errors.Is(err, ErrStatementNotFound) {
		zlog.Warn("statement not found")
		return nil, rpcstatus.Error(codes.NotFound, "Statement not found.")
	}
	if err != nil {
		zlog.Error("failed to get statement by id", zap.Error(err))
		return nil, err
	}
	return statement, nil
}
