# Maintainer: CrystalNetwork Studio
pkgname=crystallize-cli
pkgver=0.3.1
pkgrel=2
pkgdesc="CLI version of Crystallize Installer"
arch=('x86_64')
url="https://gitlab.com/crystalnetwork-studio/linux/prismlinux/os-build/crystallize"
license=('MIT')

makedepends=('go' 'make')
depends=('arch-install-scripts' 'util-linux')

build() {
  cd ..
  make build
}

package() {
  cd ..
  # Create necessary directories
  install -d "${pkgdir}/usr/bin"

  # Install the compiled binary
  install -m755 "bin/${pkgname}" "${pkgdir}/usr/bin/"
}
