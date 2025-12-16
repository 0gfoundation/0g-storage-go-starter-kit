// @title           0G Storage API Sandbox
// @version         1.0
// @description     Upload and download files using 0G Storage network. Click "Try it out" on any endpoint to test it.
// @host           localhost:8080
// @BasePath       /api/v1
// @schemes        http https

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/0gfoundation/0g-storage-client/common/blockchain"
	"github.com/0gfoundation/0g-storage-client/indexer"
	"github.com/0gfoundation/0g-storage-client/transfer"
	_ "github.com/0glabs/0g-storage-starter/docs"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/openweb3/web3go"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Network configuration for 0G Testnet
const (
	EvmRPC             = "https://evmrpc-testnet.0g.ai"
	IndexerRPCStandard = "https://indexer-storage-testnet-turbo.0g.ai"
	IndexerRPCTurbo    = "https://indexer-storage-testnet-turbo.0g.ai"
	DefaultReplicas    = 1
)

type StorageClient struct {
	web3Client    *web3go.Client
	indexerClient *indexer.Client
	ctx           context.Context
}

type UploadResponse struct {
	RootHash string `json:"root_hash"`
	TxHash   string `json:"tx_hash"`
}

// @Summary Upload a file to 0G Storage
// @Description Upload a file to 0G Storage network
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "File to upload"
// @Success 200 {object} UploadResponse
// @Router /upload [post]
func (s *Server) handleUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}

	// Create temp file
	tempFile := filepath.Join(os.TempDir(), file.Filename)
	if err := c.SaveUploadedFile(file, tempFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer os.Remove(tempFile)

	// Upload to 0G Storage
	txHash, rootHash, err := s.client.UploadFile(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, UploadResponse{
		RootHash: rootHash,
		TxHash:   txHash,
	})
}

// @Summary Download a file from 0G Storage
// @Description Download a file using its root hash
// @Produce octet-stream
// @Param root_hash path string true "Root hash of the file"
// @Success 200 {file} binary
// @Router /download/{root_hash} [get]
func (s *Server) handleDownload(c *gin.Context) {
	rootHash := c.Param("root_hash")
	if rootHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Root hash is required"})
		return
	}

	tempFile := filepath.Join(os.TempDir(), rootHash)
	defer os.Remove(tempFile)

	if err := s.client.DownloadFile(rootHash, tempFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.File(tempFile)
}

type Server struct {
	client *StorageClient
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

func (c *StorageClient) UploadFile(filePath string) (string, string, error) {
	nodes, err := c.indexerClient.SelectNodes(c.ctx, uint(DefaultReplicas), nil, "max", true)
	if err != nil {
		return "", "", fmt.Errorf("failed to select storage nodes: %v", err)
	}

	uploader, err := transfer.NewUploader(c.ctx, c.web3Client, nodes)
	if err != nil {
		return "", "", fmt.Errorf("failed to create uploader: %v", err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Minute)
	defer cancel()

	txHash, rootHash, err := uploader.UploadFile(ctx, filePath)
	if err != nil {
		return "", "", fmt.Errorf("upload failed: %v", err)
	}

	return txHash.String(), rootHash.String(), nil
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
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found. Please create one with your PRIVATE_KEY")
		log.Println("📝 Example .env file content:")
		log.Println("PRIVATE_KEY=your_private_key_here")
	}

	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		log.Fatal("❌ PRIVATE_KEY environment variable is required. Please add it to .env file")
	}

	ctx := context.Background()
	client, err := NewStorageClient(ctx, privateKey, true)
	if err != nil {
		log.Fatalf("Failed to initialize storage client: %v", err)
	}
	defer client.Close()

	server := &Server{client: client}

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/swagger/*"},
	}))

	// CORS middleware for CodeSandbox
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	v1 := r.Group("/api/v1")
	{
		v1.POST("/upload", server.handleUpload)
		v1.GET("/download/:root_hash", server.handleDownload)
	}

	// Swagger documentation endpoint with custom config
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger/doc.json")))

	// Add welcome page with better instructions
	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `
			<html>
				<head>
					<title>0G Storage API Sandbox</title>
					<meta http-equiv="refresh" content="0;url=/swagger/index.html">
				</head>
				<body>
					<p>Redirecting to Swagger UI...</p>
				</body>
			</html>
		`)
	})

	port := ":8080"
	log.Printf("🚀 Server starting on http://localhost%s", port)
	log.Printf("📚 API Documentation: http://localhost%s/swagger/index.html", port)
	log.Printf("💡 Tip: Click 'Open in New Window' in the browser preview to use Swagger UI")
	log.Fatal(r.Run(port))
}
