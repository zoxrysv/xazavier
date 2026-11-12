# Sistem Perpustakaan Sederhana (versi Streamlit)
# Cara running dengan: streamlit run nama_file.py

import streamlit as st

st.title("📚 Sistem Perpustakaan")

# Streamlit menjalankan ulang seluruh kode setiap kali kita klik sesuatu.
# session_state adalah "memori" supaya data buku tidak hilang saat itu terjadi.
if "buku_perpustakaan" not in st.session_state:
    # Dictionary: judul buku = key, True/False = tersedia/dipinjam
    st.session_state.buku_perpustakaan = {
        "Laskar Pelangi": True,
        "Bumi": True,
        "Negeri 5 Menara": True,
        "Harry Potter": True,
        "Lord of The Ring": True,
    }

buku_perpustakaan = st.session_state.buku_perpustakaan

# Menu di sisi kiri (pengganti print menu 1-5 di terminal)
pilihan = st.sidebar.radio(
    "Menu", ["Lihat Daftar Buku", "Tambah Buku", "Pinjam Buku", "Kembalikan Buku"]
)

# ---------- 1. Lihat Daftar Buku ----------
if pilihan == "Lihat Daftar Buku":
    st.subheader("Daftar Buku")

    # Perulangan for: sama persis seperti di versi terminal
    for buku, tersedia in buku_perpustakaan.items():
        if tersedia:
            st.success(f"{buku} (Tersedia)")
        else:
            st.error(f"{buku} (Dipinjam)")

# ---------- 2. Tambah Buku ----------
elif pilihan == "Tambah Buku":
    st.subheader("Tambah Buku")
    buku_baru = st.text_input("Masukkan nama buku")

    # Kode di dalam if ini jalan hanya saat tombol diklik
    if st.button("Tambah"):
        if buku_baru == "":
            st.warning("Nama buku tidak boleh kosong!")
        elif buku_baru in buku_perpustakaan:
            st.warning("Buku sudah ada!")
        else:
            buku_perpustakaan[buku_baru] = True
            st.success("Buku berhasil ditambahkan.")

# ---------- 3. Pinjam Buku ----------
elif pilihan == "Pinjam Buku":
    st.subheader("Pinjam Buku")

    # Selectbox = dropdown, jadi user tidak perlu mengetik judul
    buku = st.selectbox("Pilih buku yang ingin dipinjam", list(buku_perpustakaan.keys()))

    if st.button("Pinjam"):
        if buku_perpustakaan[buku]:
            buku_perpustakaan[buku] = False
            st.success("Buku berhasil dipinjam.")
        else:
            st.error("Maaf, buku sedang dipinjam.")

# ---------- 4. Kembalikan Buku ----------
elif pilihan == "Kembalikan Buku":
    st.subheader("Kembalikan Buku")
    buku = st.selectbox("Pilih buku yang ingin dikembalikan", list(buku_perpustakaan.keys()))

    if st.button("Kembalikan"):
        if not buku_perpustakaan[buku]:
            buku_perpustakaan[buku] = True
            st.success("Buku berhasil dikembalikan.")
        else:
            st.info("Buku memang sedang tersedia.")