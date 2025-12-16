package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/0gfoundation/0g-storage-client/common/blockchain"
	"github.com/0gfoundation/0g-storage-client/indexer"
	"github.com/0gfoundation/0g-storage-client/transfer"
	"github.com/openweb3/web3go"
)

// Network configuration for 0G Testnet
const (
	EvmRPC             = "https://evmrpc-testnet.0g.ai"
	IndexerRPCStandard = "https://indexer-storage-testnet-standard.0g.ai"
	IndexerRPCTurbo    = "https://indexer-storage-testnet-turbo.0g.ai"
	DefaultReplicas    = 1
)

type StorageClient struct {
	web3Client    *web3go.Client
	indexerClient *indexer.Client
	ctx           context.Context
}

func NewStorageClient(ctx context.Context, privateKey string, useTurbo bool) (*StorageClient, error) {
	web3Client := blockchain.MustNewWeb3(EvmRPC, privateKey)

	indexerRPC := IndexerRPCStandard
	if useTurbo {
		indexerRPC = IndexerRPCTurbo
	}

	indexerClient, err := indexer.NewClient(indexerRPC)
	if err != nil {
		web3Client.Close()
		return nil, fmt.Errorf("failed to create indexer client: %v", err)
	}

	return &StorageClient{
		web3Client:    web3Client,
		indexerClient: indexerClient,
		ctx:           ctx,
	}, nil
}

func (c *StorageClient) Close() {
	if c.web3Client != nil {
		c.web3Client.Close()
	}
}

func (c *StorageClient) UploadFile(filePath string) (string, error) {
	nodes, err := c.indexerClient.SelectNodes(c.ctx, uint(DefaultReplicas), nil, "max", true)
	if err != nil {
		return "", fmt.Errorf("failed to select storage nodes: %v", err)
	}

	uploader, err := transfer.NewUploader(c.ctx, c.web3Client, nodes)
	if err != nil {
		return "", fmt.Errorf("failed to create uploader: %v", err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Minute)
	defer cancel()

	txHash, rootHash, err := uploader.UploadFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("upload failed: %v", err)
	}
	fmt.Printf("Upload successful!\nTx hash: %s\nRoot hash: %s\n", txHash, rootHash)

	return rootHash.String(), nil
}

func (c *StorageClient) DownloadFile(rootHash, outputPath string) error {
	nodes, err := c.indexerClient.SelectNodes(c.ctx, uint(DefaultReplicas), nil, "max", true)
	if err != nil {
		return fmt.Errorf("failed to select storage nodes: %v", err)
	}

	// Combine trusted and discovered nodes for downloader
	allNodes := append(nodes.Trusted, nodes.Discovered...)
	downloader, err := transfer.NewDownloader(allNodes)
	if err != nil {
		return fmt.Errorf("failed to create downloader: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Minute)
	defer cancel()

	if err := downloader.Download(ctx, rootHash, outputPath, true); err != nil {
		return fmt.Errorf("download failed: %v", err)
	}

	return nil
}

func main() {
	// Parse command line flags
	privateKey := flag.String("key", "", "Private key for transactions (required)")
	uploadPath := flag.String("upload", "", "Path to file to upload")
	downloadHash := flag.String("download", "", "Root hash of file to download")
	outputPath := flag.String("output", "", "Path to save downloaded file")
	useTurbo := flag.Bool("turbo", false, "Use Turbo endpoint for faster but more expensive operations")
	flag.Parse()

	// Validate required flags
	if *privateKey == "" {
		log.Fatal("Private key is required. Use -key flag.")
	}

	if (*uploadPath == "") == (*downloadHash == "") {
		log.Fatal("Specify either -upload or -download (with -output), but not both or neither")
	}

	if *downloadHash != "" && *outputPath == "" {
		log.Fatal("Output path (-output) is required when downloading")
	}

	// Create storage client
	ctx := context.Background()
	client, err := NewStorageClient(ctx, *privateKey, *useTurbo)
	if err != nil {
		log.Fatalf("Failed to initialize storage client: %v", err)
	}
	defer client.Close()

	// Handle upload
	if *uploadPath != "" {
		rootHash, err := client.UploadFile(*uploadPath)
		if err != nil {
			log.Fatalf("Upload failed: %v", err)
		}
		fmt.Printf("Upload successful!\nRoot hash: %s\n", rootHash)
		return
	}

	// Handle download
	if *downloadHash != "" {
		if err := client.DownloadFile(*downloadHash, *outputPath); err != nil {
			log.Fatalf("Download failed: %v", err)
		}
		fmt.Printf("Download successful! File saved to: %s\n", *outputPath)
	}
}
