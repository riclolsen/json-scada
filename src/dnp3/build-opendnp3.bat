REM Requires https://slproweb.com/products/Win32OpenSSL.html (not the light package)
REM https://slproweb.com/download/Win64OpenSSL-1_1_1k.msi

git clone https://github.com/dnp3/opendnp3

cd opendnp3

mkdir build
cd build

SET OPENSSL_ROOT_DIR="C:\Program Files\OpenSSL-Win64"
cmake -DDNP3_DEMO=ON -DDNP3_TLS=ON -DDNP3_DOTNET=ON ..

echo now run "msbuild opendnp3.sln /p:Configuration=Release" or open opendnp3.sln on Visual Studio.