-- Token reset password.
--
-- Disimpan sebagai hash, bukan token mentah: kalau database bocor, token di
-- dalamnya tidak bisa dipakai untuk mengambil alih akun. Token mentah hanya
-- ada di email yang dikirim ke pemiliknya.
--
-- ON DELETE CASCADE karena token tidak punya arti tanpa user-nya; kalau user
-- dihapus, tokennya harus ikut hilang, bukan jadi baris yatim yang menunjuk
-- ke id yang tidak ada.
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Pencarian selalu lewat hash (dari token yang dikirim user), jadi ini yang
-- butuh index.
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_hash ON password_reset_tokens (token_hash);

-- Untuk "hapus semua token user ini" saat menerbitkan yang baru.
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens (user_id);
