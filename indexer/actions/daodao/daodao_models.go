package daodao

import (
	"time"

	"github.com/jackc/pgtype"
)

type Code struct {
	ID           int64     `gorm:"primaryKey;autoIncrement:false"`
	Height       int64     `gorm:"not null"`
	Creator      string    `gorm:"not null;default:''"`
	CreationTime time.Time `gorm:"not null"`

	Contract Contract `gorm:"foreignKey:CodeID;references:ID"`
}

type Contract struct {
	Address                string    `gorm:"primaryKey"`
	StakingContractAddress string    `gorm:"not null"`
	CodeID                 int64     `gorm:"not null"`
	Creator                string    `gorm:"not null;default:''"`
	Admin                  string    `gorm:"not null;default:''"`
	Label                  string    `gorm:"not null;default:''"`
	CreationTime           time.Time `gorm:"not null"`
	Height                 int64     `gorm:"not null"`

	DAO DAO `gorm:"foreignKey:ContractAddress;references:Address"`
}

type ExecMsg struct {
	ID      int
	Sender  string `gorm:"not null"`
	Address string `gorm:"not null"`
}

type CW20Balance struct {
	ID      int
	Address string `gorm:"not null"`
	Token   string `gorm:"not null"`
	Balance int64  `gorm:"not null"`
}

type CW20Transaction struct {
	ID               int
	CW20Address      string `gorm:"not null"`
	SenderAddress    string `gorm:"not null"`
	RecipientAddress string `gorm:"not null"`
	Amount           int64  `gorm:"not null"`
	Height           int64  `gorm:"not null"`
}

type Coin struct {
	ID int
}

type DAO struct {
	ID                     int
	ContractAddress        string `gorm:"not null"`
	StakingContractAddress string `gorm:"not null"`
	Name                   string `gorm:"not null"`
	Description            string `gorm:"not null"`
	ImageURL               string
	GovTokenID             int `gorm:"not null"`
}

type Marketing struct {
	ID            int
	Project       string
	Description   string
	MarketingText string
	LogoID        int

	GovToken GovToken `gorm:"foreignKey:MarketingID;references:ID"`
}

type GovToken struct {
	ID          int
	Address     string `gorm:"not null"`
	Name        string `gorm:"not null"`
	Symbol      string `gorm:"not null"`
	Decimals    int
	MarketingID int
}

type Logo struct {
	ID  int
	URL string
	SVG string
	PNG pgtype.Bytea
}
