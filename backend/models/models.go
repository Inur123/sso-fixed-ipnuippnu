package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	RoleSuperAdmin = "super_admin"
	RoleAnggota    = "anggota"
	StatusActive   = "active"
	StatusInactive = "inactive"

	// AccessPolicyAssignedOnly adalah default aman: hanya pengguna yang
	// ditugaskan pemilik aplikasi yang dapat menerima authorization code.
	AccessPolicyAssignedOnly   = "assigned_only"
	AccessPolicyAllActiveUsers = "all_active_users"
	ClientStatusActive         = "active"
	ClientStatusSuspended      = "suspended"
)

// User adalah identitas pusat yang digunakan oleh seluruh aplikasi klien.
type User struct {
	ID       string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Password string `gorm:"not null" json:"-"`
	Name     string `gorm:"not null" json:"name"`
	Phone    string `json:"phone"`
	Bio      string `json:"bio"`
	Gender   string `json:"gender"`
	Avatar   string `json:"avatar"`
	Role     string `gorm:"not null;default:'anggota';check:role IN ('super_admin','anggota')" json:"role"`
	IsActive bool   `gorm:"not null;default:true;index" json:"is_active"`
	// EmailVerifiedAt menjadi sumber kebenaran verifikasi. EmailVerified dan
	// Status hanya field respons agar kontrak API mudah dipakai frontend.
	EmailVerifiedAt *time.Time     `gorm:"index" json:"email_verified_at,omitempty"`
	EmailVerified   bool           `gorm:"-" json:"email_verified"`
	Status          string         `gorm:"-" json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (u *User) AfterFind(*gorm.DB) error {
	u.RefreshComputedFields()
	return nil
}

func (u *User) RefreshComputedFields() {
	u.EmailVerified = u.EmailVerifiedAt != nil
	if u.IsActive {
		u.Status = StatusActive
		return
	}
	u.Status = StatusInactive
}

// EmailVerificationOTP hanya menyimpan HMAC dari OTP. Kode enam digit tidak
// pernah disimpan mentah dan satu pengguna hanya memiliki satu OTP aktif.
type EmailVerificationOTP struct {
	ID         string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"-"`
	UserID     string    `gorm:"type:uuid;uniqueIndex;not null" json:"-"`
	User       User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	CodeHash   string    `gorm:"type:char(64);not null" json:"-"`
	Attempts   int       `gorm:"not null;default:0" json:"-"`
	LastSentAt time.Time `gorm:"not null" json:"-"`
	ExpiresAt  time.Time `gorm:"index;not null" json:"-"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}

// AuditLog menyimpan jejak tindakan keamanan dan administrasi. Payload request
// tidak pernah disimpan di sini; handler hanya memasukkan deskripsi aman yang
// sudah disanitasi. ActorID nullable untuk kejadian anonim seperti login gagal.
type AuditLog struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ActorID     *string   `gorm:"type:uuid;index:idx_audit_actor_created,priority:1" json:"actor_id,omitempty"`
	Actor       *User     `gorm:"foreignKey:ActorID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
	Action      string    `gorm:"type:varchar(80);not null;index:idx_audit_action_created,priority:1" json:"action"`
	TargetType  string    `gorm:"type:varchar(80);not null;index:idx_audit_target,priority:1" json:"target_type"`
	TargetID    string    `gorm:"type:varchar(128);index:idx_audit_target,priority:2" json:"target_id"`
	Description string    `gorm:"type:varchar(500);not null" json:"description"`
	IPAddress   string    `gorm:"type:varchar(64)" json:"ip_address"`
	Device      string    `gorm:"type:varchar(500)" json:"device"`
	Latitude    *float64  `gorm:"type:double precision" json:"latitude,omitempty"`
	Longitude   *float64  `gorm:"type:double precision" json:"longitude,omitempty"`
	Accuracy    *float64  `gorm:"type:double precision" json:"accuracy,omitempty"`
	CreatedAt   time.Time `gorm:"index:idx_audit_action_created,priority:2;index:idx_audit_actor_created,priority:2;index" json:"created_at"`
}

// Session menyimpan hash cookie agar sesi dapat dicabut tanpa menyimpan token mentah.
type Session struct {
	ID         string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TokenHash  string     `gorm:"uniqueIndex;not null" json:"-"`
	UserID     string     `gorm:"type:uuid;index;not null" json:"user_id"`
	User       User       `gorm:"foreignKey:UserID" json:"-"`
	IPAddress  string     `json:"ip_address"`
	UserAgent  string     `json:"user_agent"`
	LastSeenAt time.Time  `gorm:"not null" json:"last_seen_at"`
	ExpiresAt  time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt  *time.Time `gorm:"index" json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// OAuthClient adalah relying party yang terdaftar. Hash bcrypt digunakan untuk
// autentikasi client, sedangkan ciphertext AES-GCM hanya untuk fitur reveal
// terotorisasi milik pemilik aplikasi. Record lama dapat belum memiliki ciphertext.
type OAuthClient struct {
	ID               string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"client_id"`
	SecretHash       string         `gorm:"not null" json:"-"`
	SecretCiphertext *string        `gorm:"type:text" json:"-"`
	SecretVersion    uint64         `gorm:"not null;default:1" json:"-"`
	Name             string         `gorm:"not null" json:"name"`
	Description      string         `json:"description"`
	RedirectURIs     string         `gorm:"not null" json:"-"`
	AllowedScopes    string         `gorm:"not null;default:'email openid profile'" json:"-"`
	AccessPolicy     string         `gorm:"not null;default:'assigned_only';check:access_policy IN ('assigned_only','all_active_users')" json:"access_policy"`
	Status           string         `gorm:"not null;default:'active';index;check:status IN ('active','suspended')" json:"status"`
	OwnerID          string         `gorm:"type:uuid;index;not null" json:"owner_id"`
	Owner            User           `gorm:"foreignKey:OwnerID" json:"-"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// OAuthClientAssignment adalah keputusan admission sederhana: pengguna boleh
// atau tidak boleh masuk ke satu aplikasi. Otorisasi bisnis berada di RP.
type OAuthClientAssignment struct {
	ID        string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ClientID  string         `gorm:"type:uuid;not null;uniqueIndex:idx_client_assignment_user;index" json:"client_id"`
	Client    OAuthClient    `gorm:"foreignKey:ClientID;constraint:OnDelete:CASCADE" json:"-"`
	UserID    string         `gorm:"type:uuid;not null;uniqueIndex:idx_client_assignment_user;index" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	IsActive  bool           `gorm:"not null;default:true;index" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ProvisioningOutbox menjamin perubahan assignment tidak hilang ketika
// aplikasi tujuan sedang tidak tersedia. Payload tidak menyimpan kredensial;
// secret penandatangan selalu dibaca dari environment saat pengiriman.
type ProvisioningOutbox struct {
	ID            string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ClientID      string     `gorm:"type:uuid;not null;index:idx_provisioning_ready,priority:2" json:"client_id"`
	EventType     string     `gorm:"type:varchar(80);not null" json:"event_type"`
	Subject       string     `gorm:"type:uuid;not null;index" json:"subject"`
	Payload       string     `gorm:"type:jsonb;not null" json:"-"`
	Status        string     `gorm:"type:varchar(20);not null;default:'pending';index:idx_provisioning_ready,priority:1" json:"status"`
	Attempts      int        `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt time.Time  `gorm:"not null;index:idx_provisioning_ready,priority:3" json:"next_attempt_at"`
	LockedUntil   *time.Time `gorm:"index" json:"-"`
	LastError     string     `gorm:"type:varchar(500)" json:"last_error,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	DedupeKey     *string    `gorm:"type:varchar(255);uniqueIndex" json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// OAuthAuthCode adalah authorization code sekali pakai. Nilai code disimpan sebagai hash.
type OAuthAuthCode struct {
	ID                  string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CodeHash            string    `gorm:"uniqueIndex;not null"`
	ClientID            string    `gorm:"type:uuid;index;not null"`
	UserID              string    `gorm:"type:uuid;index;not null"`
	RedirectURI         string    `gorm:"not null"`
	Scope               string    `gorm:"not null"`
	CodeChallenge       string    `gorm:"not null"`
	CodeChallengeMethod string    `gorm:"not null;default:'S256'"`
	State               string    `json:"-"`
	Nonce               string    `json:"-"`
	AuthTime            time.Time `gorm:"not null" json:"-"`
	ExpiresAt           time.Time `gorm:"index;not null"`
	CreatedAt           time.Time
}

// OAuthToken mencatat grant aktif, access-token JTI, dan refresh token yang diputar.
type OAuthToken struct {
	ID               string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AccessJTI        string     `gorm:"uniqueIndex;not null"`
	RefreshTokenHash string     `gorm:"uniqueIndex;not null"`
	FamilyID         string     `gorm:"type:varchar(64);index;not null"`
	ClientID         string     `gorm:"type:uuid;index;not null"`
	UserID           string     `gorm:"type:uuid;index;not null"`
	Scope            string     `gorm:"not null"`
	ExpiresAt        time.Time  `gorm:"index;not null"`
	RefreshExpiresAt time.Time  `gorm:"index;not null"`
	LastUsedAt       *time.Time `json:"-"`
	RevokedAt        *time.Time `gorm:"index" json:"-"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
