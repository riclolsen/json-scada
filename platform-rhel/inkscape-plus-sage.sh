# to compile inkscape

# INSTALL SCRIPT FOR JSON-SCADA ON RHEL9 AND COMPATIBLE PLATFORMS
# username is supposed to be jsonscada
JS_USERNAME=jsonscada

# to compile inkscape
sudo dnf -y install ninja-build ccache libjpeg-devel libxslt-devel gtkmm30-devel gspell-devel boost-devel poppler-devel poppler-glib-devel gtest-devel harfbuzz-devel 
sudo dnf -y install libwpg-devel librevenge-devel libvisio-devel libcdr-devel readline-devel ImageMagick-c++-devel GraphicsMagick-c++-devel
sudo dnf -y install pango-devel gsl-devel libsoup-devel lcms2-devel gc-devel double-conversion-devel potrace python3-scour
sudo dnf -y install https://dl.rockylinux.org/pub/rocky/9/devel/$(arch)/os/Packages/p/potrace-devel-1.16-7.el9.0.1.$(arch).rpm
sudo dnf -y install https://dl.rockylinux.org/pub/rocky/9/devel/$(arch)/os/Packages/l/ladspa-1.13-28.el9.$(arch).rpm

cd /home/jsonscada
sudo -u $JS_USERNAME sh -c 'git clone https://gitlab.com/ricardolo/inkscape-scadavis-editor.git'
cd inkscape-scadavis-editor
git checkout 1.4.x-scada
sudo -u $JS_USERNAME sh -c 'git submodule init && git submodule update --recursive'
sudo -u $JS_USERNAME sh -c 'wget -v https://gitlab.com/inkscape/inkscape-ci-docker/-/raw/master/install_dependencies.sh -O install_dependencies.sh'
sudo bash install_dependencies.sh --recommended

sudo -u $JS_USERNAME sh -c 'mkdir build'
cd build

# to compile on Windows with msys2, use -DCMAKE_CXX_STANDARD=20
# to compile on Linux, use -DCMAKE_CXX_STANDARD=17

#sudo -u $JS_USERNAME sh -c 'cmake -DENABLE_POPPLER_CAIRO=OFF -DCMAKE_CXX_STANDARD=17 ..'
#sudo -u $JS_USERNAME sh -c 'make'
#sudo make install
sudo -u $JS_USERNAME sh -c 'cmake -DCMAKE_INSTALL_PREFIX="${PWD}/install_dir" -DENABLE_POPPLER=OFF -DCMAKE_C_COMPILER_LAUNCHER=ccache -DCMAKE_CXX_COMPILER_LAUNCHER=ccache -DCMAKE_BUILD_TYPE=Release -DWITH_INTERNAL_2GEOM=ON -DCMAKE_EXPORT_COMPILE_COMMANDS=ON -G Ninja ..'
sudo -u $JS_USERNAME sh -c 'ninja -j4'
sudo -u $JS_USERNAME sh -c 'ninja install'

echo "Start custom Inkscape with:"
echo "~/inkscape-scadavis-editor/build/install_dir/bin/inkscape &"
