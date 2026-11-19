# to compile inkscape

# INSTALL SCRIPT FOR JSON-SCADA ON UBUNTU AND COMPATIBLE PLATFORMS
# username is supposed to be jsonscada
JS_USERNAME=jsonscada

sudo apt remove -y inkscape
# Inkscape build dependencies
sudo apt -y install ninja-build libjpeg-dev libxslt-dev libgtkmm-3.0-dev libboost-all-dev \
    libgtest-dev libharfbuzz-dev libwpg-dev librevenge-dev libvisio-dev \
    libcdr-dev libreadline-dev libmagick++-dev libgraphicsmagick++1-dev libpango1.0-dev libgsl-dev \
    libsoup2.4-dev liblcms2-dev libgc-dev libdouble-conversion-dev potrace python3-scour lib2geom-dev
sudo apt -y install libgspell-1-dev libgspell-1-2 libpotrace-dev libnss3-dev libgpgmepp-dev libcurlpp-dev

cd /home/jsonscada

# install poppler dependencies from apt
sudo apt -y install libpoppler-dev libpoppler-glib-dev libpoppler-private-dev
# or install poppler from source
#  sudo -u $JS_USERNAME sh -c 'wget -v https://poppler.freedesktop.org/poppler-26.04.0.tar.xz -O poppler.tar.xz'
#  sudo -u $JS_USERNAME sh -c 'tar -xf poppler.tar.xz'
#  cd poppler-26.04.0
#  sudo -u $JS_USERNAME sh -c 'mkdir build'
#  cd build
#  sudo -u $JS_USERNAME sh -c 'cmake .. -DENABLE_GPGME=OFF -DENABLE_QT6=OFF  -DCMAKE_INSTALL_PREFIX=/usr/local -DCMAKE_BUILD_TYPE=release'
#  sudo make install

cd /home/jsonscada
sudo -u $JS_USERNAME sh -c 'git clone https://gitlab.com/ricardolo/inkscape-scadavis-editor.git'
cd inkscape-scadavis-editor
sudo -u $JS_USERNAME sh -c 'git checkout 1.4.x-scada'
sudo -u $JS_USERNAME sh -c 'git submodule init && git submodule update --recursive'

sudo -u $JS_USERNAME sh -c 'wget -v https://gitlab.com/inkscape/inkscape-ci-docker/-/raw/master/install_dependencies.sh -O install_dependencies.sh'
sudo bash install_dependencies.sh --recommended

sudo -u $JS_USERNAME sh -c 'mkdir build'
cd build

# to compile on Windows with msys2, use -DCMAKE_CXX_STANDARD=20
# to compile on Linux, use -DCMAKE_CXX_STANDARD=17

# use -DENABLE_POPPLER=OFF to disable poppler (pdf import) that can cause compilation issues
#sudo -u $JS_USERNAME sh -c 'cmake -DENABLE_POPPLER_CAIRO=OFF -DCMAKE_CXX_STANDARD=17 ..'
#sudo -u $JS_USERNAME sh -c 'make'
#sudo make install
sudo -u $JS_USERNAME sh -c 'cmake -DCMAKE_INSTALL_PREFIX="${PWD}/install_dir" -DENABLE_POPPLER=OFF -DCMAKE_C_COMPILER_LAUNCHER=ccache -DCMAKE_CXX_COMPILER_LAUNCHER=ccache -DCMAKE_BUILD_TYPE=Release -DWITH_INTERNAL_2GEOM=ON -DCMAKE_EXPORT_COMPILE_COMMANDS=ON -G Ninja ..'
sudo -u $JS_USERNAME sh -c 'ninja -j4'
sudo -u $JS_USERNAME sh -c 'ninja -j4'
sudo -u $JS_USERNAME sh -c 'ninja install'

echo "Start custom Inkscape with:"
echo "~/inkscape-scadavis-editor/build/install_dir/bin/inkscape &"
