package server

import (
	"errors"
	"net/http"

	"github.com/10664kls/estatement/internal/statement"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	edpb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	statement *statement.Service
}

func NewServer(statement *statement.Service) (*Server, error) {
	if statement == nil {
		return nil, errors.New("statement service is nil")
	}

	s := &Server{
		statement: statement,
	}
	return s, nil
}

func (s *Server) Install(e *echo.Echo, mdw ...echo.MiddlewareFunc) error {
	if e == nil {
		return errors.New("echo is nil")
	}

	v1 := e.Group("/v1", mdw...)

	v1.GET("/statements", s.listStatements)
	v1.GET("/statements/:id", s.getStatementByID)

	v1.GET("/product-names", s.listProductNames)
	v1.GET("/occupations", s.listOccupations)
	v1.GET("/terms", s.listTerms)

	return nil
}

// badJSON is a helper function to create an error when c.Bind return an error.
func badJSON() error {
	s, _ := status.New(codes.InvalidArgument, "Request body must be a valid JSON.").
		WithDetails(&edpb.ErrorInfo{
			Reason: "BINDING_ERROR",
			Domain: "http",
		})
	zap.L().Error("failed to bind json", zap.Error(s.Err()))
	return s.Err()
}

func (s *Server) listStatements(c echo.Context) error {
	req := new(statement.StatementQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	statements, err := s.statement.ListStatements(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, statements)
}

func (s *Server) getStatementByID(c echo.Context) error {
	id := c.Param("id")

	statement, err := s.statement.GetStatementByID(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"statement": statement,
	})
}

func (s *Server) listProductNames(c echo.Context) error {
	productNames, err := s.statement.ListProductNames(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"productNames": productNames,
	})
}

func (s *Server) listOccupations(c echo.Context) error {
	occupations, err := s.statement.ListOccupations(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"occupations": occupations,
	})
}

func (s *Server) listTerms(c echo.Context) error {
	terms, err := s.statement.ListTerms(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"terms": terms,
	})
}
