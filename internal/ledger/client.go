package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/cirfis/go-TaxBandit/internal/config"
	pb "github.com/darcys22/godbledger/proto"
	"google.golang.org/grpc"
)

type Client struct {
	grpcEndpoint string
}

func NewClient(endpoint string) *Client {
	return &Client{grpcEndpoint: endpoint}
}

func (c *Client) CommitPayroll(cfg *config.Configuration) error {
	conn, err := grpc.Dial(c.grpcEndpoint, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		return fmt.Errorf("database unreachable at %s", c.grpcEndpoint)
	}
	defer conn.Close()
	db := pb.NewLedgerClient(conn)

	var splits []*pb.Split
	splits = append(splits, &pb.Split{AccountId: "Expenses:Payroll:GrossWages", Amount: cfg.PayrollParameters.GrossWageCents})
	splits = append(splits, &pb.Split{AccountId: "Assets:RelayChecking", Amount: -cfg.PayrollParameters.NetPaycheckCents})

	for _, tax := range cfg.StatutoryTaxes {
		splits = append(splits, &pb.Split{AccountId: tax.LedgerAccount, Amount: -tax.AmountCents})
	}

	transaction := &pb.TransactionRequest{
		Date:        time.Now().Format("2006-01-02"),
		Description: fmt.Sprintf("%s Payroll Invariant - %s", cfg.CompanyProfile.LegalName, time.Now().Format("January 2006")),
		Splits:      splits,
	}

	_, err = db.AddTransaction(context.Background(), transaction)
	return err
}

func (c *Client) VerifyAnnualGross(expectedCents int64) error {
	conn, err := grpc.Dial(c.grpcEndpoint, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		return fmt.Errorf("database unreachable for audit")
	}
	defer conn.Close()
	db := pb.NewLedgerClient(conn)

	resp, err := db.GetAccountBalance(context.Background(), &pb.ReportRequest{AccountId: "Expenses:Payroll:GrossWages"})
	if err != nil {
		return fmt.Errorf("could not query gross wages")
	}

	if resp.Balance != expectedCents {
		return fmt.Errorf("math invariant failed: expected %d cents, found %d cents", expectedCents, resp.Balance)
	}
	return nil
}
