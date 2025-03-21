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
	zlog := s.zlog.With(
		zap.String("method", "ListStatements"),
		zap.Any("query", in),
	)

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
	zlog := s.zlog.With(
		zap.String("method", "GetStatementByID"),
		zap.Any("id", id),
	)

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

func (s *Service) ListProductNames(ctx context.Context) ([]string, error) {
	zlog := s.zlog.With(zap.Any("method", "ListProductNames"))

	zlog.Info("starting to list product names")

	productNames, err := listProductNames(ctx, s.db)
	if err != nil {
		zlog.Error("failed to list product names", zap.Error(err))
		return nil, err
	}
	return productNames, nil
}

func (s *Service) ListOccupations(ctx context.Context) ([]string, error) {
	zlog := s.zlog.With(zap.Any("method", "ListOccupations"))

	zlog.Info("starting to list occupations")

	occupations, err := listOccupations(ctx, s.db)
	if err != nil {
		zlog.Error("failed to list occupations", zap.Error(err))
		return nil, err
	}
	return occupations, nil
}

func (s *Service) ListTerms(ctx context.Context) ([]string, error) {
	zlog := s.zlog.With(zap.Any("method", "ListTerms"))

	zlog.Info("starting to list terms")

	terms, err := listTerms(ctx, s.db)
	if err != nil {
		zlog.Error("failed to list terms", zap.Error(err))
		return nil, err
	}
	return terms, nil
}
