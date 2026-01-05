# to compile inkscape

# INSTALL SCRIPT FOR JSON-SCADA ON UBUNTU AND COMPATIBLE PLATFORMS
# username is supposed to be jsonscada
JS_USERNAME=jsonscada

# Inkscape build dependencies
sudo apt -y install ninja-build libjpeg-dev libxslt-dev libgtkmm-3.0-dev libboost-all-dev \
    libpoppler-dev libpoppler-glib-dev libgtest-dev libharfbuzz-dev libwpg-dev librevenge-dev libvisio-dev \
    libcdr-dev libreadline-dev libmagick++-dev libgraphicsmagick++1-dev libpango1.0-dev libgsl-dev \
    libsoup2.4-dev liblcms2-dev libgc-dev libdouble-conversion-dev potrace python3-scour
sudo apt -y install libgspell-1-dev libgspell-1-2 libpotrace-dev libpoppler-private-dev

cd /home/jsonscada
sudo -u $JS_USERNAME sh -c 'git clone --recurse-submodules https://gitlab.com/ricardolo/inkscape-rebased.git'
cd inkscape-rebased
sudo -u $JS_USERNAME sh -c 'mkdir build'
cd build

# to compile on Windows with msys2, use -DCMAKE_CXX_STANDARD=20
# to compile on Linux, use -DCMAKE_CXX_STANDARD=17

#sudo -u $JS_USERNAME sh -c 'cmake -DENABLE_POPPLER_CAIRO=OFF -DCMAKE_CXX_STANDARD=17 ..'
#sudo -u $JS_USERNAME sh -c 'make'
#sudo make install
sudo -u $JS_USERNAME sh -c 'cmake -G Ninja -DENABLE_POPPLER_CAIRO=OFF -DCMAKE_CXX_STANDARD=17 ..'
sudo -u $JS_USERNAME sh -c 'ninja -j4'
sudo ninja install