#variable_watch(CMAKE_SYSTEM_NAME)
cmake_minimum_required (VERSION 3.11)
set(DNP3_MAJOR_VERSION 0)
set(DNP3_MINOR_VERSION 1)
set(DNP3_MICRO_VERSION 0)
set(DNP3_VERSION ${DNP3_MAJOR_VERSION}.${DNP3_MINOR_VERSION}.${DNP3_MICRO_VERSION})
project(Dnp3Server VERSION ${DNP3_VERSION})

add_executable(Dnp3Server ./main.cpp ./json.hpp)
set_target_properties(Dnp3Server PROPERTIES FOLDER cpp/json-scada-drivers)
target_compile_features(Dnp3Server PRIVATE cxx_std_17)
target_include_directories (Dnp3Server PRIVATE 
../../mongo-cxx-driver-lib/include
../../mongo-cxx-driver-lib/include/mongocxx/v_noabi
../../mongo-cxx-driver-lib/include/bsoncxx/v_noabi
../../dnp3/opendnp3/cpp/lib/include
)

find_package(OpenSSL 1.1.1 REQUIRED)

link_directories(../../dnp3/opendnp3/cpp/lib)

set(LSB_RELEASE_ID_SHORT "")
find_program(LSB_RELEASE_EXEC lsb_release)
execute_process(COMMAND ${LSB_RELEASE_EXEC} -is
    OUTPUT_VARIABLE LSB_RELEASE_ID_SHORT
    OUTPUT_STRIP_TRAILING_WHITESPACE
)

# Use the MSVC variable to determine if this is a Windows build.
set(WINDOWS_BUILD ${MSVC})


if (WINDOWS_BUILD OR NOT CMAKE_SYSTEM_NAME MATCHES "Linux")

if (WINDOWS_BUILD)
# WINDOWS MSVC

set(CMAKE_EXE_LINKER_FLAGS
    "${CMAKE_EXE_LINKER_FLAGS} /NODEFAULTLIB:LIBCMT")

target_link_libraries (Dnp3Server 
PUBLIC
  ../../../mongo-cxx-driver-lib/lib/bsoncxx-static-rts-x64-v143-md
  ../../../mongo-cxx-driver-lib/lib/mongocxx-static-rts-x64-v143-md
  ../../../mongo-cxx-driver-lib/lib/mongoc-static-1.0
  ../../../mongo-cxx-driver-lib/lib/bson-static-1.0
  ../../../dnp3/opendnp3/build/cpp/lib/release/opendnp3
  OpenSSL::SSL
  OpenSSL::Crypto
  advapi32.lib
  bcrypt.lib
  comdlg32.lib
  crypt32.lib
  dnsapi.lib
  gdi32.lib
  kernel32.lib
  ole32.lib
  oleaut32.lib
  secur32.lib
  shell32.lib
  user32.lib
  uuid.lib
  winspool.lib
  ws2_32.lib
)
else ()
# MSYS2

add_library(libmongocxx STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libmongocxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libmongocxx-static.a)

add_library(libbsoncxx STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libbsoncxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libbsoncxx-static.a)

add_library(libmongoc STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libmongoc PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libmongoc-static-1.0.a)

add_library(libbson STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libbson PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libbson-static-1.0.a)

add_library(libopendnp3 STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libopendnp3 PROPERTIES IMPORTED_LOCATION ../../opendnp3/build/cpp/lib/libopendnp3.a)


target_link_libraries (Dnp3Server
PUBLIC
  libmongocxx
  libbsoncxx
  libmongoc
  libbson
  libopendnp3
  zstd
  OpenSSL::SSL
  OpenSSL::Crypto
  advapi32.lib
  bcrypt.lib
  comdlg32.lib
  crypt32.lib
  dnsapi.lib
  gdi32.lib
  kernel32.lib
  ole32.lib
  oleaut32.lib
  secur32.lib
  shell32.lib
  user32.lib
  uuid.lib
  winspool.lib
  ws2_32.lib
  wsock32.lib
)

endif ()

else ()

# LINUX

# OR LSB_RELEASE_ID_SHORT MATCHES ""

if (LSB_RELEASE_ID_SHORT MATCHES "ubuntu") 

# UBUNTU LINUX

add_library(libmongocxx STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libmongocxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libmongocxx-static.a)

add_library(libbsoncxx STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libbsoncxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libbsoncxx-static.a)

add_library(libmongoc STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libmongoc PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libmongoc-static-1.0.a)

add_library(libbson STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libbson PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libbson-static-1.0.a)

add_library(libopendnp3 STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libopendnp3 PROPERTIES IMPORTED_LOCATION ../../opendnp3/build/cpp/lib/libopendnp3.so)

else()

# OTHER LINUX

add_library(libmongocxx STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libmongocxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib64/libmongocxx-static.a)

add_library(libbsoncxx STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libbsoncxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib64/libbsoncxx-static.a)

add_library(libmongoc STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libmongoc PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib64/libmongoc-static-1.0.a)

add_library(libbson STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libbson PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib64/libbson-static-1.0.a)

add_library(libopendnp3 STATIC IMPORTED)
# Provide the full path to the library, so CMake knows where to find it.
set_target_properties(libopendnp3 PROPERTIES IMPORTED_LOCATION ../../opendnp3/build/cpp/lib/libopendnp3.so)

endif()

# LINUX 

target_link_libraries (Dnp3Server 
PUBLIC
  libmongocxx
  libbsoncxx
  libmongoc
  libbson
  libopendnp3
  OpenSSL::SSL
  OpenSSL::Crypto
  zstd
  resolv
  sasl2
)

endif ()

install(TARGETS Dnp3Server RUNTIME DESTINATION bin)
