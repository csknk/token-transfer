package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"

	bin "github.com/gagliardetto/binary"
	solanago "github.com/gagliardetto/solana-go"
	ata "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

var (
	sender   string
	receiver string
	network  string
	amount   uint64
)

const (
	// this is the locally stored private key of the sender
	signerKeyPath   = "/home/david/.config/solana/id.json"
	programIDBase58 = "3WyacwnCNiz4Q1PedWyuwodYpLFu75jrhgRTZp69UcA9" // mockrock
)

func init() {
	flag.StringVar(&network, "network", "localnet", "Network to broadcast to: devnet|mainnet")
	flag.StringVar(&receiver, "receiver", "", "Receiver's base58 public key (required)")
	flag.Uint64Var(&amount, "amount", 0, "Amount to mint (required)")
}

func main() {
	flag.Parse()
	if receiver == "" {
		log.Fatal("--receiver flag is required")
	}
	if amount == 0 {
		log.Fatal("--amount flag is required")
	}

	endpoint := map[string]string{
		"devnet":  "https://api.devnet.solana.com",
		"mainnet": "https://api.mainnet-beta.solana.com",
	}[network]

	if endpoint == "" {
		log.Fatal("Invalid network. Use devnet or mainnet")
	}

	rpcClient := rpc.New(rpc.DevNet_RPC)
	wsClient, err := ws.Connect(context.Background(), rpc.DevNet_WS)
	if err != nil {
		panic(err)
	}

	receiverKey, err := solanago.PublicKeyFromBase58(receiver)
	if err != nil {
		log.Fatalf("invalid receiver: %v", err)
	}
	accountFrom, err := solanago.PrivateKeyFromSolanaKeygenFile(signerKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	tx, err := BuildTokenTransferTransaction(accountFrom.PublicKey(), receiverKey, programIDBase58, amount, rpcClient)
	if err != nil {
		log.Fatal(err)
	}

	tx.Sign(
		func(key solanago.PublicKey) *solanago.PrivateKey {
			if accountFrom.PublicKey().Equals(key) {
				return &accountFrom
			}
			return nil
		},
	)
	sig, err := confirm.SendAndConfirmTransaction(
		context.TODO(),
		rpcClient,
		wsClient,
		tx,
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", sig)
}

func BuildTokenTransferTransaction(sender solanago.PublicKey, receiver solanago.PublicKey, programIDBase58 string, amount uint64, client *rpc.Client) (*solanago.Transaction, error) {
	programID := solanago.MustPublicKeyFromBase58(programIDBase58)

	mintAddress, err := GetMintAddress(programID)
	if err != nil {
		return nil, fmt.Errorf("can't get mint address: %v", err)
	}

	mint, err := GetMint(context.Background(), client, mintAddress, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("error getting mint: %v", err)
	}

	amountToTransfer := amount * uint64(math.Pow(10, float64(mint.Decimals)))

	recentBlockHash, err := client.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("can't get recent block hash: %v", err)
	}

	instructions := []solanago.Instruction{}

	senderAta, _, err := solanago.FindAssociatedTokenAddress(sender, mintAddress)
	if err != nil {
		return nil, fmt.Errorf("can't get ATA for sender %s: %v", sender.String(), err)
	}

	receiverAta, _, err := solanago.FindAssociatedTokenAddress(receiver, mintAddress)
	if err != nil {
		return nil, fmt.Errorf("can't get ATA for receiver %s: %v", receiver.String(), err)
	}

	// This is needed because the receiver needs a token account (ATA) - if it does not have one, our transfer
	// transaction needs to create one using the NewCreateInstruction method.
	recipientTokenAccount, err := client.GetAccountInfo(context.Background(), receiverAta)
	if err != nil || recipientTokenAccount == nil || len(recipientTokenAccount.Value.Data.GetBinary()) == 0 {
		instructions = append(
			instructions,
			ata.NewCreateInstruction(
				sender,
				receiver,
				mintAddress,
			).Build(),
		)
	}

	// The actual token transfer instruction
	instructions = append(
		instructions,
		token.NewTransferInstruction(
			amountToTransfer,
			senderAta,
			receiverAta,
			sender,
			[]solanago.PublicKey{},
		).Build(),
	)
	return solanago.NewTransaction(
		instructions,
		recentBlockHash.Value.Blockhash,
		solanago.TransactionPayer(sender))
}

// GetMintAddress calculates a Program Derived Address (PDA) to serve as a mint address for a token based on a given token
// symbol and program ID. Note that the seeds must match those used when the program was initialised. There must be
// consistency between the seedds used here and how and the seeds used during on-chain PDA generation.
func GetMintAddress(programID solanago.PublicKey) (solanago.PublicKey, error) {
	seeds := [][]byte{
		[]byte("wrapped_mint"),
	}
	addr, _, err := solanago.FindProgramAddress(seeds, programID)
	if err != nil {
		return solanago.PublicKey{}, err
	}
	return addr, nil
}

func GetMint(context context.Context, client *rpc.Client, mintPubkey solanago.PublicKey, commitment rpc.CommitmentType) (token.Mint, error) {
	accountInfo, err := GetAccountInfo(context, client, mintPubkey, commitment)
	if err != nil {
		return token.Mint{}, err
	}

	data := accountInfo.Value.Data.GetBinary()

	var mint token.Mint

	err = bin.NewBorshDecoder(data).Decode(&mint)
	if err != nil {
		return token.Mint{}, err
	}

	return mint, nil
}

func GetAccountInfo(ctx context.Context, client *rpc.Client, account solanago.PublicKey, commitment rpc.CommitmentType) (out *rpc.GetAccountInfoResult, err error) {
	return client.GetAccountInfoWithOpts(
		ctx,
		account,
		&rpc.GetAccountInfoOpts{
			Commitment: commitment,
			DataSlice:  nil,
		},
	)
}
