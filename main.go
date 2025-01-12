package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ANSI escape codes for coloring
const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Magenta    = "\033[35m"
	Cyan       = "\033[36m"
	BoldYellow = "\033[1;33m"
	BoldCyan   = "\033[1;36m"
	BoldGreen  = "\033[1;32m"
	BoldRed    = "\033[1;31m"
	BoldBlue   = "\033[1;34m" // Menambahkan definisi BoldBlue
)

// Block represents each block in the blockchain
type Block struct {
	Index        int    `json:"index"`
	Timestamp    string `json:"timestamp"`
	Data         string `json:"data"`
	Nonce        uint64 `json:"nonce"`
	Hash         string `json:"hash"`
	PreviousHash string `json:"previous_hash"`
	Difficulty   int    `json:"difficulty"` // **Field Difficulty ditambahkan**
}

// calculateHash calculates the SHA-256 hash of a block's contents
func calculateHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + block.Data + strconv.FormatUint(block.Nonce, 10) + block.PreviousHash
	hash := sha256.New()
	hash.Write([]byte(record))
	return hex.EncodeToString(hash.Sum(nil))
}

// createGenesisBlock creates the first block in the blockchain by mining it with default difficulty
func createGenesisBlock(difficulty int) Block {
	fmt.Println(BoldYellow + "Membuat blok genesis melalui proses mining..." + Reset)

	// Blok Dummy dengan Index=-1 dan PreviousHash=64 nol
	dummyBlock := Block{
		Index:        -1,
		Timestamp:    "",
		Data:         "",
		Nonce:        0,
		Hash:         "0000000000000000000000000000000000000000000000000000000000000000",
		PreviousHash: "",
	}

	// Mine Genesis Block dengan menggunakan dummyBlock sebagai previousBlock
	genesisBlock := mineBlock("Genesis Block", dummyBlock, difficulty)
	return genesisBlock
}

// saveBlock saves a block as a JSON file
func saveBlock(block Block) error {
	// Pastikan direktori "blocks" ada
	if _, err := os.Stat("blocks"); os.IsNotExist(err) {
		err := os.Mkdir("blocks", os.ModePerm)
		if err != nil {
			return err
		}
	}

	filename := fmt.Sprintf("block%d.json", block.Index)
	filePath := filepath.Join("blocks", filename)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(block)
}

// loadBlockchain loads the blockchain from JSON files
func loadBlockchain() ([]Block, error) {
	var blockchain []Block

	// Pastikan direktori "blocks" ada
	if _, err := os.Stat("blocks"); os.IsNotExist(err) {
		return blockchain, nil // Tidak ada blok yang disimpan
	}

	files, err := filepath.Glob("blocks/block*.json")
	if err != nil {
		return blockchain, err
	}

	// Sort files berdasarkan index
	sort.Slice(files, func(i, j int) bool {
		var indexI, indexJ int
		fmt.Sscanf(filepath.Base(files[i]), "block%d.json", &indexI)
		fmt.Sscanf(filepath.Base(files[j]), "block%d.json", &indexJ)
		return indexI < indexJ
	})

	for _, file := range files {
		var block Block
		f, err := os.Open(file)
		if err != nil {
			return blockchain, err
		}

		decoder := json.NewDecoder(f)
		if err := decoder.Decode(&block); err != nil {
			f.Close()
			return blockchain, err
		}
		f.Close()
		blockchain = append(blockchain, block)
	}

	return blockchain, nil
}

// mineBlock performs the mining process to find a valid nonce
func mineBlock(data string, previousBlock Block, difficulty int) Block {
	var wg sync.WaitGroup
	result := make(chan Block)
	done := make(chan struct{})
	nonceChan := make(chan uint64, 100) // Buffer untuk nonce
	numCPU := runtime.NumCPU()

	wg.Add(numCPU)

	// Fungsi mining yang dijalankan oleh setiap goroutine
	mining := func(start uint64, step uint64) {
		defer wg.Done()
		var nonce uint64 = start
		prefix := strings.Repeat("0", difficulty)

		for {
			select {
			case <-done:
				return
			default:
				// Membuat blok dengan nonce saat ini
				newBlock := Block{
					Index:        previousBlock.Index + 1,
					Timestamp:    time.Now().Format(time.RFC3339),
					Data:         data,
					Nonce:        nonce,
					Hash:         "",
					PreviousHash: previousBlock.Hash,
					Difficulty:   difficulty, // **Menetapkan Difficulty**
				}
				newBlock.Hash = calculateHash(newBlock)

				// Memeriksa apakah hash memenuhi tingkat kesulitan
				if strings.HasPrefix(newBlock.Hash, prefix) {
					// Mengirim hasil melalui channel
					result <- newBlock
					return
				}

				// Mengirim nonce terkini secara periodik
				if nonce%100000 == 0 {
					select {
					case nonceChan <- nonce:
					default:
						// Jika channel penuh, abaikan untuk mencegah blocking
					}
				}

				// Meningkatkan nonce sesuai langkah
				nonce += step
			}
		}
	}

	// Meluncurkan goroutine mining
	for i := 0; i < numCPU; i++ {
		go mining(uint64(i), uint64(numCPU))
	}

	// Goroutine untuk menampilkan nonce secara dinamis
	var monitorWg sync.WaitGroup
	monitorWg.Add(1)
	go func() {
		defer monitorWg.Done()
		lastNonce := uint64(0)
		for nonce := range nonceChan {
			if nonce > lastNonce {
				// Menggunakan format string konstan dengan placeholders
				fmt.Printf("\r%sNonce sedang diperiksa: %d%s", BoldCyan, nonce, Reset)
				lastNonce = nonce
			}
		}
	}()

	// Menunggu salah satu goroutine menemukan nonce yang valid
	foundBlock := <-result
	close(done)
	wg.Wait()

	// Menutup channel nonceChan setelah semua goroutine selesai
	close(nonceChan)
	monitorWg.Wait()

	fmt.Println() // Menambahkan newline setelah mining selesai

	return foundBlock
}

// displayBlockchain prints all the blocks in the blockchain
func displayBlockchain(blockchain []Block) {
	fmt.Println(BoldYellow + "\n=== Blockchain ===" + Reset)
	for _, block := range blockchain {
		fmt.Println(BoldGreen + "-------------------------------------------------" + Reset)
		fmt.Printf("%sIndex         :%s %d\n", BoldCyan, Reset, block.Index)
		fmt.Printf("%sTimestamp     :%s %s\n", BoldCyan, Reset, block.Timestamp)
		fmt.Printf("%sData          :%s %s\n", BoldCyan, Reset, block.Data)
		fmt.Printf("%sNonce         :%s %d\n", BoldCyan, Reset, block.Nonce)
		fmt.Printf("%sHash          :%s %s\n", BoldCyan, Reset, block.Hash)
		fmt.Printf("%sPreviousHash  :%s %s\n", BoldCyan, Reset, block.PreviousHash)
		fmt.Printf("%sDifficulty    :%s %d\n", BoldCyan, Reset, block.Difficulty) // **Menampilkan Difficulty**
	}
	fmt.Println(BoldGreen + "-------------------------------------------------" + Reset)
}

// isBlockchainValid checks the integrity of the blockchain
func isBlockchainValid(blockchain []Block) bool {
	for i, block := range blockchain {
		// Validasi hash
		if block.Hash != calculateHash(block) {
			fmt.Printf(Red+"Invalid hash at block %d\n"+Reset, block.Index)
			return false
		}

		// Validasi tingkat kesulitan berdasarkan Difficulty setiap blok
		prefix := strings.Repeat("0", block.Difficulty)
		if !strings.HasPrefix(block.Hash, prefix) {
			fmt.Printf(Red+"Block %d does not meet difficulty requirements\n"+Reset, block.Index)
			return false
		}

		// Validasi PreviousHash (kecuali untuk Genesis Block)
		if i > 0 {
			if block.PreviousHash != blockchain[i-1].Hash {
				fmt.Printf(Red+"Previous hash mismatch at block %d\n"+Reset, block.Index)
				return false
			}
		} else {
			// Validasi Genesis Block's PreviousHash
			expectedPrevHash := "0000000000000000000000000000000000000000000000000000000000000000"
			if block.PreviousHash != expectedPrevHash {
				fmt.Printf(Red + "Invalid PreviousHash for Genesis Block\n" + Reset)
				return false
			}
		}
	}

	fmt.Println(Green + "Blockchain is valid." + Reset)
	return true
}

// menuDisplay displays the interactive menu
func menuDisplay() {
	fmt.Println(BoldYellow + "\n=== Menu Blockchain ===" + Reset)
	fmt.Println(BoldBlue + "1. Tambah Blok Baru" + Reset)
	fmt.Println(BoldBlue + "2. Tampilkan Blockchain" + Reset)
	fmt.Println(BoldBlue + "3. Set Tingkat Kesulitan" + Reset)
	fmt.Println(BoldBlue + "4. Validasi Blockchain" + Reset) // **Opsi Baru**
	fmt.Println(BoldBlue + "5. Keluar" + Reset)              // **Menyesuaikan nomor opsi**
	fmt.Print(BoldCyan + "Pilih opsi: " + Reset)
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	var blockchain []Block
	var err error
	currentDifficulty := 5 // Default difficulty

	// Memuat blockchain jika ada, atau membuat genesis block
	blockchain, err = loadBlockchain()
	if err != nil {
		fmt.Println(Red+"Error loading blockchain:"+Reset, err)
		return
	}

	if len(blockchain) == 0 {
		genesisBlock := createGenesisBlock(currentDifficulty)
		blockchain = append(blockchain, genesisBlock)
		// Menyimpan blok genesis
		if err := saveBlock(genesisBlock); err != nil {
			fmt.Println(Red+"Error menyimpan blok genesis:"+Reset, err)
			return
		}
		fmt.Println(Green + "Blok genesis berhasil dibuat dan ditambahkan ke blockchain." + Reset)
	} else {
		// Menentukan tingkat kesulitan saat ini berdasarkan blok terakhir
		lastBlock := blockchain[len(blockchain)-1]
		currentDifficulty = lastBlock.Difficulty // **Mengambil Difficulty dari blok terakhir**
		fmt.Printf(Green+"Blockchain ditemukan dengan %d blok. Tingkat kesulitan saat ini: %d\n"+Reset, len(blockchain), currentDifficulty)
	}

	for {
		menuDisplay()
		option, _ := reader.ReadString('\n')
		option = strings.TrimSpace(option)

		switch option {
		case "1":
			// Input data untuk blok baru
			fmt.Print(BoldCyan + "Masukkan data (teks) yang akan di-mining: " + Reset)
			data, _ := reader.ReadString('\n')
			data = strings.TrimSpace(data)

			// Gunakan tingkat kesulitan saat ini
			fmt.Printf(BoldYellow+"Menggunakan tingkat kesulitan saat ini: %d\n"+Reset, currentDifficulty)

			fmt.Println(BoldYellow + "\nMemulai proses mining..." + Reset)
			previousBlock := blockchain[len(blockchain)-1]
			startTime := time.Now()
			newBlock := mineBlock(data, previousBlock, currentDifficulty)
			elapsed := time.Since(startTime)

			// Menambahkan blok baru ke blockchain
			blockchain = append(blockchain, newBlock)

			// Menyimpan blok baru sebagai file JSON
			if err := saveBlock(newBlock); err != nil {
				fmt.Println(Red+"Error menyimpan blok:"+Reset, err)
				continue
			}

			fmt.Println(Green + "Blok baru berhasil ditambahkan:" + Reset)
			fmt.Printf("%sIndex         :%s %d\n", BoldCyan, Reset, newBlock.Index)
			fmt.Printf("%sNonce         :%s %d\n", BoldCyan, Reset, newBlock.Nonce)
			fmt.Printf("%sHash          :%s %s\n", BoldCyan, Reset, newBlock.Hash)
			fmt.Printf("%sPreviousHash  :%s %s\n", BoldCyan, Reset, newBlock.PreviousHash)
			fmt.Printf("%sDifficulty    :%s %d\n", BoldCyan, Reset, newBlock.Difficulty)
			fmt.Printf("%sWaktu         :%s %s\n", BoldCyan, Reset, elapsed)

		case "2":
			// Tampilkan seluruh blockchain
			if len(blockchain) == 0 {
				fmt.Println(Yellow + "Blockchain masih kosong." + Reset)
			} else {
				displayBlockchain(blockchain)
			}

		case "3":
			// Set tingkat kesulitan
			fmt.Print(BoldCyan + "Masukkan tingkat kesulitan baru (jumlah nol di awal hash): " + Reset)
			difficultyInput, _ := reader.ReadString('\n')
			difficultyInput = strings.TrimSpace(difficultyInput)
			newDifficulty, err := strconv.Atoi(difficultyInput)
			if err != nil || newDifficulty < 0 {
				fmt.Println(Red + "Tingkat kesulitan harus berupa angka non-negatif." + Reset)
				continue
			}
			currentDifficulty = newDifficulty
			fmt.Printf(Green+"Tingkat kesulitan berhasil diubah menjadi %d.\n"+Reset, currentDifficulty)

		case "4":
			// Validasi Blockchain
			fmt.Println(BoldYellow + "Memvalidasi blockchain..." + Reset)
			isBlockchainValid(blockchain)

		case "5":
			// Keluar dari program
			fmt.Println(Yellow + "Keluar dari program." + Reset)
			return

		default:
			fmt.Println(Red + "Opsi tidak valid. Silakan pilih opsi yang tersedia." + Reset)
		}
	}
}
