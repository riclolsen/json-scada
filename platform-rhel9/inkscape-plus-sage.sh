# to compile inkscape

# INSTALL SCRIPT FOR JSON-SCADA ON RHEL9 AND COMPATIBLE PLATFORMS
# username is supposed to be jsonscada
JS_USERNAME=jsonscada

# to compile inkscape
sudo dnf -y install ninja-build libjpeg-devel libxslt-devel gtkmm30-devel gspell-devel boost-devel poppler-devel poppler-glib-devel gtest-devel harfbuzz-devel 
sudo dnf -y install libwpg-devel librevenge-devel libvisio-devel libcdr-devel readline-devel ImageMagick-c++-devel GraphicsMagick-c++-devel
sudo dnf -y install pango-devel gsl-devel libsoup-devel lcms2-devel gc-devel double-conversion-devel potrace python3-scour
sudo dnf -y install https://dl.rockylinux.org/pub/rocky/9/devel/$(arch)/os/Packages/p/potrace-devel-1.16-7.el9.$(arch).rpm
sudo dnf -y install https://dl.rockylinux.org/pub/rocky/9/devel/$(arch)/os/Packages/l/ladspa-1.13-28.el9.$(arch).rpm

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