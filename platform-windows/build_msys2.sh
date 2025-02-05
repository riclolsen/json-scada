cd ../src/dnp3/opendnp3
mkdir build
cd build
cmake -DDNP3_EXAMPLES=ON -DDNP3_TLS=ON -DOPENSSL_ROOT_DIR="d:/msys64/mingw64" -DOPENSSL_USE_STATIC_LIBS=TRUE ..
cmake --build . --config Release

cd ../../mongo-cxx-driver/mongo-cxx-driver/build
cmake .. -DCMAKE_INSTALL_PREFIX="../../../mongo-cxx-driver-lib" -DCMAKE_CXX_STANDARD=17 -DPython3_ROOT_DIR=/mingw64/lib/python3.12 -DBUILD_VERSION=4.0.0 -DBUILD_SHARED_LIBS=OFF -DBUILD_SHARED_AND_STATIC_LIBS=OFF
cmake --build . --config RelWithDebInfo
cmake --build . --target install --config RelWithDebInfo

cd ../../../dnp3/Dnp3Server
mkdir build
cd build
cmake ..
cmake --build . --config Release

cp *.exe ../../../dnp3/Dnp3Server/bin
cp /mingw64/bin/libgcc_s_seh-1.dll ../../../dnp3/Dnp3Server/bin
cp /mingw64/bin/libwinpthread-1.dll ../../../dnp3/Dnp3Server/bin
cp /mingw64/bin/libzstd.dll ../../../dnp3/Dnp3Server/bin
cp /mingw64/bin/libstdc++-6.dll ../../../dnp3/Dnp3Server/bin
cp /mingw64/bin/libcrypto-3-x64.dll ../../../dnp3/Dnp3Server/bin
cp /mingw64/bin/libssl-3-x64.dll ../../../dnp3/Dnp3Server/bin
