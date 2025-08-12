#!/bin/bash

###### Function to replace tabs with spaces in all .cs files################
format_cs_files() {
  echo "Formatting .cs files: Replacing tabs with spaces..."
  find . -type f -name "*.cs" -exec sed -i 's/\t/    /g' {} +
  echo "Formatting completed!"
}

###### Function to replace tabs with spaces in all .cs files################
format_c_files() {
  echo "Formatting .c files: Replacing tabs with spaces..."
  find . -type f -name "*.c" -exec sed -i 's/\t/    /g' {} +
  echo "Formatting completed!"
}

#########Clean directory#########################
clean_build_directories()
{
  echo "Removing .vs/, bin/, obj/ folders, any .git directories or files, and doxydoc.NET folder..."
  find "$FOLDER_NAME" -type d \( -name "vs" -o -name "bin" -o -name "obj" -o -name "doxydoc.NET" -o -name ".git" \) -exec rm -rf {} +
  find "$FOLDER_NAME" -type f -name ".git" -exec rm -f {} +
  echo "Cleanup completed!"
}

##########Create release folder#####################
create_release_folder()
{
	# Print the value
	echo "Creating folder: $FOLDER_NAME"

	# Create the folder
	mkdir -p "$FOLDER_NAME"

	echo "Folder '$FOLDER_NAME' created successfully!"

	cp -rf config $FOLDER_NAME
	cp -rf demos $FOLDER_NAME
	cp -rf dotnet $FOLDER_NAME
	cp -rf examples $FOLDER_NAME
	cp -rf fuzz $FOLDER_NAME
	cp -rf hal $FOLDER_NAME
	cp -rf pyiec61850 $FOLDER_NAME
	cp -rf src $FOLDER_NAME
	cp -rf tools $FOLDER_NAME
	cp -rf CHANGELOG $FOLDER_NAME
	cp -rf CMakeLists.txt $FOLDER_NAME
	cp -rf Makefile $FOLDER_NAME
	cp -rf COPYING $FOLDER_NAME
	cp -rf mingw-w64-x86_64.cmake $FOLDER_NAME
	cp -rf README.md $FOLDER_NAME
	cp -rf SECURITY.md $FOLDER_NAME
	
}

################ Function to create a tar.gz archive############################
compress_to_tar() {
  ARCHIVE_NAME="$FOLDER_NAME.tar.gz"
  echo "Creating archive: $ARCHIVE_NAME"
  tar -czf "$ARCHIVE_NAME" -C "$(dirname "$FOLDER_NAME")" "$(basename "$FOLDER_NAME")"
  echo "Archive '$ARCHIVE_NAME' created successfully!"
}

# Wait for user input if arguments are missing
while [ -z "$1" ]; do
  read -p "Enter version: " VERSION_NAME_INPUT
  set -- "$VERSION_NAME_INPUT" "$2"
done

while [ -z "$2" ]; do
  read -p "Enter option ([1]release/[2]formatFiles/[3]all): " OPTION_INPUT
  set -- "$1" "$OPTION_INPUT"
done

# Store arguments
PREFIX="../libiec61850-"
FOLDER_NAME="${PREFIX}${1}"
OPTION="$2"

# Execute option case
case "$OPTION" in
  1)
    create_release_folder
    ;;
2)
	format_cs_files
	format_c_files
	;;
  3)
	format_cs_files
	format_c_files
	create_release_folder
	clean_build_directories
	compress_to_tar
    ;;
  *)
    echo "Invalid option. Use 'prepare', 'release', or 'delete'."
    exit 1
    ;;
esac


#####################################################
