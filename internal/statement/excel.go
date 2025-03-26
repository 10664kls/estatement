package statement

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

func (s *Service) GenExcel(ctx context.Context, in *BatchGetStatementReq) (*bytes.Buffer, error) {
	zlog := s.zlog.With(
		zap.String("method", "GenExcel"),
		zap.Any("query", in),
	)

	zlog.Info("starting to gen excel")

	fx := excelize.NewFile()
	defer fx.Close()

	const sheetName = "Statement Requests"

	sheet, err := fx.NewSheet("Statement Requests")
	if err != nil {
		zlog.Error("failed to create sheet", zap.Error(err))
		return nil, err
	}

	fx.SetActiveSheet(sheet)

	// add header
	fx.SetCellValue(sheetName, "A1", "CUID")
	fx.SetCellValue(sheetName, "B1", "CusNum")
	fx.SetCellValue(sheetName, "C1", "CusName")
	fx.SetCellValue(sheetName, "D1", "AccNo")
	fx.SetCellValue(sheetName, "E1", "Term")
	fx.SetCellValue(sheetName, "F1", "BankName")
	fx.SetCellValue(sheetName, "G1", "CreateDate")
	fx.SetCellValue(sheetName, "H1", "CreateBy")
	fx.SetCellValue(sheetName, "I1", "BankStatus")
	fx.SetCellValue(sheetName, "J1", "BankMoreInfo")
	fx.SetCellValue(sheetName, "K1", "BankCreateDate")
	fx.SetCellValue(sheetName, "L1", "Gender")
	fx.SetCellValue(sheetName, "M1", "ProductName")
	fx.SetCellValue(sheetName, "N1", "EmailStatus")
	fx.SetCellValue(sheetName, "O1", "EmailMsg")
	fx.SetCellValue(sheetName, "P1", "Occupation")
	fx.SetCellValue(sheetName, "Q1", "StatusBanking")

	row := 2
	var nextID string
	for {
		statements, err := batchGetStatements(ctx, s.db, 200, nextID, in)
		if err != nil {
			zlog.Error("failed to batch get statements", zap.Error(err))
			return nil, err
		}

		if len(statements) == 0 {
			break
		}

		s.mu.Lock()
		nextID = statements[len(statements)-1].ID
		s.mu.Unlock()

		for _, s := range statements {
			var bankCreatedAt, bankStatus, bankMoreInfo,
				mailStatus, mailMsg string
			if s.BankAccount.CreatedAt != nil {
				bankCreatedAt = s.BankAccount.CreatedAt.Format("02/01/2006 15:04:05")
			}

			if s.BankAccount.Status != nil {
				bankStatus = *s.BankAccount.Status
			}
			if s.BankAccount.Info != nil {
				bankMoreInfo = *s.BankAccount.Info
			}

			if s.Email.IsSent != nil {
				mailStatus = *s.Email.IsSent
			}
			if s.Email.Message != nil {
				mailMsg = *s.Email.Message
			}
			fx.SetCellValue(sheetName, fmt.Sprintf("A%d", row), s.ID)
			fx.SetCellValue(sheetName, fmt.Sprintf("B%d", row), s.QueueNumber)
			fx.SetCellValue(sheetName, fmt.Sprintf("C%d", row), s.Customer.DisplayName)
			fx.SetCellValue(sheetName, fmt.Sprintf("D%d", row), s.BankAccount.Number)
			fx.SetCellValue(sheetName, fmt.Sprintf("E%d", row), s.BankAccount.Term)
			fx.SetCellValue(sheetName, fmt.Sprintf("F%d", row), s.BankAccount.Code)
			fx.SetCellValue(sheetName, fmt.Sprintf("G%d", row), s.CreatedAt.Format("02/01/2006 15:04:05"))
			fx.SetCellValue(sheetName, fmt.Sprintf("H%d", row), s.CreatedBy)
			fx.SetCellValue(sheetName, fmt.Sprintf("I%d", row), bankStatus)
			fx.SetCellValue(sheetName, fmt.Sprintf("J%d", row), bankMoreInfo)
			fx.SetCellValue(sheetName, fmt.Sprintf("K%d", row), bankCreatedAt)
			fx.SetCellValue(sheetName, fmt.Sprintf("L%d", row), s.Customer.Gender)
			fx.SetCellValue(sheetName, fmt.Sprintf("M%d", row), s.ProductName)
			fx.SetCellValue(sheetName, fmt.Sprintf("N%d", row), mailStatus)
			fx.SetCellValue(sheetName, fmt.Sprintf("O%d", row), mailMsg)
			fx.SetCellValue(sheetName, fmt.Sprintf("P%d", row), s.Customer.Occupation)
			fx.SetCellValue(sheetName, fmt.Sprintf("Q%d", row), s.Status)
			row++
		}
	}

	buf, err := fx.WriteToBuffer()
	if err != nil {
		zlog.Error("failed to write file to buffer", zap.Error(err))
		return nil, err
	}

	return buf, nil
}
