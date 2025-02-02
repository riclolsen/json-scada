REM Requires https://slproweb.com/products/Win32OpenSSL.html (not the light package)
REM https://slproweb.com/download/Win64OpenSSL-3_4_0.msi

# git clone https://github.com/dnp3/opendnp3

cd opendnp3

mkdir build
cd build

cmake -DDNP3_EXAMPLES=ON -DDNP3_TLS=ON -DOPENSSL_ROOT_DIR="C:\Program Files\OpenSSL-Win64" -DOPENSSL_USE_STATIC_LIBS=TRUE -DOPENSSL_MSVC_STATIC_RT=TRUE ..

echo now run "msbuild opendnp3.sln /p:Configuration=Release" or open opendnp3.sln on Visual Studio.

cd ..\..\
cd Dnp3Server
mkdir build
cd build
cmake -DOPENSSL_ROOT_DIR="C:\Program Files\OpenSSL-Win64" -DOPENSSL_USE_STATIC_LIBS=TRUE -DOPENSSL_MSVC_STATIC_RT=TRUE ..
msbuild Dnp3Server.sln /p:Configuration=Release