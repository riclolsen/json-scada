cmake_minimum_required(VERSION 3.11)

set(DNP3_CLIENT_MAJOR_VERSION 0)
set(DNP3_CLIENT_MINOR_VERSION 1)
set(DNP3_CLIENT_MICRO_VERSION 0)
set(DNP3_CLIENT_VERSION
    ${DNP3_CLIENT_MAJOR_VERSION}.${DNP3_CLIENT_MINOR_VERSION}.${DNP3_CLIENT_MICRO_VERSION})

project(Dnp3ClientCpp VERSION ${DNP3_CLIENT_VERSION})

add_executable(Dnp3ClientCpp
    ./main.cpp
    ./utils.cpp
    ./soe_handler.cpp
    ./connection.cpp
    ./autocreate.cpp
    ./mongo.cpp
)
set_target_properties(Dnp3ClientCpp PROPERTIES FOLDER cpp/json-scada-drivers)
target_compile_features(Dnp3ClientCpp PRIVATE cxx_std_17)
target_include_directories(
    Dnp3ClientCpp PRIVATE
    ../../mongo-cxx-driver-lib/include
    ../../mongo-cxx-driver-lib/include/mongocxx/v_noabi
    ../../mongo-cxx-driver-lib/include/bsoncxx/v_noabi
    ../../dnp3/opendnp3/cpp/lib/include
    ../Dnp3Server
)

find_package(OpenSSL 1.1.1 REQUIRED)

link_directories(../../dnp3/opendnp3/cpp/lib)

set(WINDOWS_BUILD ${MSVC})

if(WINDOWS_BUILD OR NOT CMAKE_SYSTEM_NAME MATCHES "Linux")

  if(WINDOWS_BUILD)
  # WINDOWS MSVC
    set(CMAKE_EXE_LINKER_FLAGS
        "${CMAKE_EXE_LINKER_FLAGS} /NODEFAULTLIB:LIBCMT")

    target_link_libraries(
        Dnp3ClientCpp
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
        ws2_32.lib)
  else()
  # MSYS2
    add_library(libmongocxx STATIC IMPORTED)
    set_target_properties(libmongocxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libmongocxx-static.a)

    add_library(libbsoncxx STATIC IMPORTED)
    set_target_properties(libbsoncxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libbsoncxx-static.a)

    add_library(libmongoc STATIC IMPORTED)
    set_target_properties(libmongoc PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libmongoc-static-1.0.a)

    add_library(libbson STATIC IMPORTED)
    set_target_properties(libbson PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libbson-static-1.0.a)

    add_library(libopendnp3 STATIC IMPORTED)
    set_target_properties(libopendnp3 PROPERTIES IMPORTED_LOCATION ../../opendnp3/build/cpp/lib/libopendnp3.a)

    target_link_libraries(
        Dnp3ClientCpp
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
        wsock32.lib)
  endif()

else()

# LINUX

  if(EXISTS "/etc/os-release")
    file(READ "/etc/os-release" OS_RELEASE_CONTENT)
    string(REGEX MATCH "ID=ubuntu" IS_UBUNTU "${OS_RELEASE_CONTENT}")
  endif()

  if(IS_UBUNTU)
    add_library(libmongocxx STATIC IMPORTED)
    set_target_properties(libmongocxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libmongocxx-static.a)

    add_library(libbsoncxx STATIC IMPORTED)
    set_target_properties(libbsoncxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libbsoncxx-static.a)

    add_library(libmongoc STATIC IMPORTED)
    set_target_properties(libmongoc PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libmongoc-static-1.0.a)

    add_library(libbson STATIC IMPORTED)
    set_target_properties(libbson PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib/libbson-static-1.0.a)

    add_library(libopendnp3 STATIC IMPORTED)
    set_target_properties(libopendnp3 PROPERTIES IMPORTED_LOCATION ../../opendnp3/build/cpp/lib/libopendnp3.so)
  else()
    add_library(libmongocxx STATIC IMPORTED)
    set_target_properties(libmongocxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib64/libmongocxx-static.a)

    add_library(libbsoncxx STATIC IMPORTED)
    set_target_properties(libbsoncxx PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib64/libbsoncxx-static.a)

    add_library(libmongoc STATIC IMPORTED)
    set_target_properties(libmongoc PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib64/libmongoc-static-1.0.a)

    add_library(libbson STATIC IMPORTED)
    set_target_properties(libbson PROPERTIES IMPORTED_LOCATION ../../../mongo-cxx-driver-lib/lib64/libbson-static-1.0.a)

    add_library(libopendnp3 STATIC IMPORTED)
    set_target_properties(libopendnp3 PROPERTIES IMPORTED_LOCATION ../../opendnp3/build/cpp/lib/libopendnp3.so)
  endif()

  target_link_libraries(
      Dnp3ClientCpp
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
      sasl2)
endif()

install(TARGETS Dnp3ClientCpp RUNTIME DESTINATION bin)
