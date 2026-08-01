package database

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// FileRepository مدیریت ذخیره‌سازی ticket ها در فایل JSON
type FileRepository struct {
	filePath string
	tickets  []int64
	mu       sync.RWMutex
}

// NewFileRepository ایجاد یا بارگذاری فایل ticket ها
func NewFileRepository(filePath string) (*FileRepository, error) {
	repo := &FileRepository{
		filePath: filePath,
		tickets:  make([]int64, 0),
	}

	// اگر فایل وجود دارد، آن را بارگذاری کن
	if _, err := os.Stat(filePath); err == nil {
		if err := repo.loadFromFile(); err != nil {
			return nil, err
		}
		log.Printf("✅ Loaded %d tickets from %s", len(repo.tickets), filePath)
	} else {
		// اگر فایل وجود ندارد، یک فایل خالی ایجاد کن
		if err := repo.saveToFile(); err != nil {
			return nil, err
		}
		log.Printf("📁 Created new tickets file: %s", filePath)
	}

	return repo, nil
}

// SaveTicket ذخیره یک ticket جدید
func (r *FileRepository) SaveTicket(ticket int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// بررسی تکراری نبودن
	for _, t := range r.tickets {
		if t == ticket {
			log.Printf("⚠️ Ticket %d already exists, skipping", ticket)
			return nil
		}
	}

	r.tickets = append(r.tickets, ticket)
	log.Printf("💾 Saved ticket %d (total: %d)", ticket, len(r.tickets))

	return r.saveToFile()
}

// GetAllTickets دریافت تمام ticket های ذخیره شده
func (r *FileRepository) GetAllTickets() []int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// کپی آرایه برای جلوگیری از race condition
	result := make([]int64, len(r.tickets))
	copy(result, r.tickets)
	return result
}

// RemoveTicket حذف یک ticket از لیست
func (r *FileRepository) RemoveTicket(ticket int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	found := false
	newTickets := make([]int64, 0, len(r.tickets))

	for _, t := range r.tickets {
		if t == ticket {
			found = true
			continue
		}
		newTickets = append(newTickets, t)
	}

	if !found {
		log.Printf("⚠️ Ticket %d not found for removal", ticket)
		return nil
	}

	r.tickets = newTickets
	log.Printf("🗑️ Removed ticket %d (remaining: %d)", ticket, len(r.tickets))

	return r.saveToFile()
}

// Exists بررسی وجود یک ticket
func (r *FileRepository) Exists(ticket int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, t := range r.tickets {
		if t == ticket {
			return true
		}
	}
	return false
}

// Count تعداد ticket های ذخیره شده
func (r *FileRepository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tickets)
}

// loadFromFile بارگذاری ticket ها از فایل JSON
func (r *FileRepository) loadFromFile() error {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		r.tickets = make([]int64, 0)
		return nil
	}

	if err := json.Unmarshal(data, &r.tickets); err != nil {
		return err
	}

	return nil
}

// saveToFile ذخیره ticket ها در فایل JSON
func (r *FileRepository) saveToFile() error {
	data, err := json.MarshalIndent(r.tickets, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.filePath, data, 0644)
}
