-- Flag admin.
--
-- Kenapa kolom di tabel users, bukan daftar di env:
--   Siapa yang boleh mengelola akun adalah data, bukan konfigurasi — ia
--   berubah saat seseorang dipromosikan, dan perubahan itu harus tercatat
--   bersama barisnya. Env juga berarti dua sumber kebenaran yang bisa
--   berbeda antara dev dan prod tanpa ada yang menyadarinya.
--
-- DEFAULT FALSE supaya migrasi ini tidak diam-diam memberi hak istimewa ke
-- akun yang sudah ada; promosi harus eksplisit.
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- Bootstrap: akun pemilik yayasan dipromosikan saat migrasi pertama kali
-- jalan, supaya selalu ada satu admin yang bisa membuka halaman pengguna.
-- Idempoten — menjalankan ulang tidak mengubah apa pun.
UPDATE users SET is_admin = TRUE WHERE lower(email) = 'abuamar.albadawi@gmail.com';

-- Partial index: yang dicari hanya baris admin, dan jumlahnya sedikit.
-- Index penuh akan menyimpan hampir semua baris (mayoritas non-admin) dan
-- tidak memberi banyak.
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users (is_admin) WHERE is_admin;
