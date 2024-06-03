# to compile inkscape

# INSTALL SCRIPT FOR JSON-SCADA ON RHEL9 AND COMPATIBLE PLATFORMS
# username is supposed to be jsonscada
JS_USERNAME=jsonscada

cd /home/jsonscada
sudo -u $JS_USERNAME sh -c 'git clone --recurse-submodules https://gitlab.com/ricardolo/inkscape-rebased.git'
cd inkscape-rebased
sudo -u $JS_USERNAME sh -c 'mkdir build'
cd build
#sudo -u $JS_USERNAME sh -c 'cmake -DENABLE_POPPLER_CAIRO=OFF -DCMAKE_CXX_STANDARD=20 ..'
#sudo -u $JS_USERNAME sh -c 'make'
#sudo make install
sudo -u $JS_USERNAME sh -c 'cmake -G Ninja -DENABLE_POPPLER_CAIRO=OFF -DCMAKE_CXX_STANDARD=20 ..'
sudo -u $JS_USERNAME sh -c 'ninja -j4'
sudo ninja install