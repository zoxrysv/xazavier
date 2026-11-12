# Simple Library Management System

# Create a dictionary to store the list of books.
# The book title is the key, while True/False represents the book's availability.
# True = Available, False = Borrowed.
buku_perpustakaan = {
    "Laskar Pelangi": True,
    "Bumi": True,
    "Negeri 5 Menara": True,
    "Harry Potter": True,
    "Lord of The Ring": True
}

# Use an infinite loop to keep the program running until the user chooses to exit.
while True:

    # Display the main menu of the library management system.
    print("\n===== SISTEM PERPUSTAKAAN =====")
    print("1. Lihat Daftar Buku")
    print("2. Tambah Buku")
    print("3. Pinjam Buku")
    print("4. Kembalikan Buku")
    print("5. Keluar")

    # Ask the user to select a menu option from 1 to 5.
    # input() receives user input as a string.
    pilihan = input("Pilih menu (1-5): ")

    # If the user selects option 1, display the book list.
    if pilihan == "1":

        # Display the book list heading.
        print("\nDaftar Buku:")

        # Use .items() to retrieve each book title and its availability status.
        # The for loop processes every book in the dictionary.
        for buku, tersedia in buku_perpustakaan.items():

            # Check the availability status using a conditional expression.
            # If True, the status is "Tersedia".
            # If False, the status is "Dipinjam".
            status = "Tersedia" if tersedia else "Dipinjam"

            # Display the book title and its current status.
            print(f"- {buku} ({status})")

    # If the user selects option 2, add a new book.
    elif pilihan == "2":

        # Ask the user to enter the title of the new book.
        buku_baru = input("Masukkan nama buku: ")

        # Check whether the book already exists in the dictionary.
        if buku_baru in buku_perpustakaan:

            # Display a message if the book already exists.
            print("Buku sudah ada!")

        else:

            # Add the new book to the dictionary.
            # Set its status to True because the new book is available.
            buku_perpustakaan[buku_baru] = True

            # Display a success message.
            print("Buku berhasil ditambahkan.")

    # If the user selects option 3, process a book borrowing request.
    elif pilihan == "3":

        # Ask the user to enter the title of the book they want to borrow.
        buku = input("Masukkan nama buku yang ingin dipinjam: ")

        # Check whether the requested book exists in the dictionary.
        if buku in buku_perpustakaan:

            # Check whether the book is available (True).
            if buku_perpustakaan[buku]:

                # Change the book status to False because it has been borrowed.
                buku_perpustakaan[buku] = False

                # Display a message confirming the successful borrowing.
                print("Buku berhasil dipinjam.")

            else:

                # Display a message if the book has already been borrowed.
                print("Maaf, buku sedang dipinjam.")

        else:

            # Display a message if the requested book does not exist.
            print("Buku tidak ditemukan.")

    # If the user selects option 4, process a book return request.
    elif pilihan == "4":

        # Ask the user to enter the title of the book they want to return.
        buku = input("Masukkan nama buku yang ingin dikembalikan: ")

        # Check whether the book exists in the dictionary.
        if buku in buku_perpustakaan:

            # Check whether the book is currently borrowed (False).
            if not buku_perpustakaan[buku]:

                # Change the book status to True because it has been returned.
                buku_perpustakaan[buku] = True

                # Display a message confirming the successful return.
                print("Buku berhasil dikembalikan.")

            else:

                # Display a message if the book is already available.
                print("Buku memang sedang tersedia.")

        else:

            # Display a message if the requested book does not exist.
            print("Buku tidak ditemukan.")

    # If the user selects option 5, exit the program.
    elif pilihan == "5":

        # Display a thank-you message before closing the program.
        print("Terima kasih telah menggunakan sistem perpustakaan.")

        # Stop the while loop and terminate the program.
        break

    else:

        # Display an error message if the user enters an invalid menu option.
        print("Pilihan tidak valid!")