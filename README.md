# 0G Storage Go SDK Starter Kit

A starter kit demonstrating how to use the 0G Storage Go SDK for decentralized file storage. This example implements a simple CLI tool showcasing the core SDK functionalities.

## Repository Branches

### 1. Master Branch
REST API implementation using Gin framework with Swagger documentation.
```bash
git checkout master
```

- Features:
  - RESTful endpoints for upload/download
  - Swagger UI for API testing

### 2. CLI Version Branch (Current)
Command-line interface implementation available in the cli-version branch.
```bash
git checkout cli-version
```

- Features:
  - Direct file upload/download via CLI
  - Command-line flags for configuration

## Core Components (CLI Version)
```go
import (
    "github.com/0gfoundation/0g-storage-client/common/blockchain"  // Web3 client wrapper
    "github.com/0gfoundation/0g-storage-client/indexer"           // Node selection and management
    "github.com/0gfoundation/0g-storage-client/transfer"          // File upload/download operations
)
```

## Upload Process

### 1. Initialize Clients
The first step is to create the necessary clients for blockchain interaction and node management:
```go
// Create Web3 client for blockchain transactions
web3Client := blockchain.MustNewWeb3(EvmRPC, privateKey)

// Create indexer client for node selection
indexerClient, _ := indexer.NewClient(indexerRPC)
```
This sets up:
- Web3 client for signing and sending transactions
- Indexer client for managing storage node selection

### 2. Node Selection & File Preparation
Select storage nodes and prepare the file for upload:
```go
// Select available storage nodes
nodes, _ := indexerClient.SelectNodes(ctx, uint(DefaultReplicas), nil, "max", true)

// Create uploader with selected nodes
uploader, _ := transfer.NewUploader(ctx, web3Client, nodes)
```
During this phase:
- Available nodes are queried from the indexer service
- Nodes are selected based on availability and performance
- File is prepared for chunked upload
- Merkle tree is constructed for file verification

### 3. Upload Operation
Execute the upload with blockchain transaction:
```go
// Upload file and get transaction/root hashes
txHash, rootHash, _ := uploader.UploadFile(ctx, filePath)
fmt.Printf("Upload successful!\nTx hash: %s\nRoot hash: %s\n", txHash, rootHash)
```
This process:
- Creates and signs a storage transaction
- Splits file into chunks for parallel upload
- Uploads chunks to selected nodes
- Verifies storage confirmation from nodes

## Download Process

### 1. Node Discovery & Preparation
Locate nodes storing the file:
```go
// Find nodes storing the file
nodes, _ := indexerClient.SelectNodes(ctx, uint(DefaultReplicas), nil, "max", true)

// Combine trusted and discovered nodes for downloader
allNodes := append(nodes.Trusted, nodes.Discovered...)
downloader, _ := transfer.NewDownloader(allNodes)
```
This step:
- Uses root hash to find storage nodes
- Selects optimal nodes for download
- Prepares verification structure

### 2. Download Operation
Retrieve and verify the file:
```go
// Download with verification
err := downloader.Download(ctx, rootHash, outputPath, true)
if err == nil {
    fmt.Printf("Download successful! File saved to: %s\n", outputPath)
}
```
The process includes:
- Parallel chunk download from nodes
- Continuous integrity verification
- Automatic retry on node failures
- Final file reassembly and verification

## Quick Start Examples

### Upload a file
```bash
go run main.go -key YOUR_PRIVATE_KEY -upload path/to/file
# Output:
# Upload successful!
# Tx hash: 0xabc... (blockchain transaction)
# Root hash: 0xdef... (file identifier)
```

### Download a file
```bash
go run main.go -key YOUR_PRIVATE_KEY -download ROOT_HASH -output path/to/save
# Output:
# Download successful! File saved to: ./downloaded.txt
```

## Command Options
- `-key`: Private key for signing transactions (required)
- `-upload`: Path to the file to upload
- `-download`: Root hash of the file to retrieve
- `-output`: Destination path for downloaded file
- `-turbo`: Use high-performance nodes (costs more)

## Network Configuration
- EVM RPC Endpoint: `https://evmrpc-testnet.0g.ai`
- Standard Indexer: `https://indexer-storage-testnet-standard.0g.ai`
- Turbo Indexer: `https://indexer-storage-testnet-turbo.0g.ai`

## Best Practices
- Always close the web3Client using defer
- Use context with timeout for operations
- Verify file integrity during downloads
- Handle errors appropriately
- Clean up temporary resources

## Next Steps
Explore more SDK features in the [0G Storage Client documentation](https://github.com/0gfoundation/0g-storage-client). Learn more about the [0G Storage Network](https://docs.0g.ai/0g-storage).
