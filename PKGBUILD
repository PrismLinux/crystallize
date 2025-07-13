# Maintainer: CrystalNetwork Studio
pkgname=crystallize-cli
pkgver=0.1.2
pkgrel=1
pkgdesc="CLI version of Crystallize Installer"
arch=('x86_64')
url="https://gitlab.com/crystalnetwork-studio/linux/prismlinux/os-build/crystallize"
license=('MIT')

makedepends=('rust' 'cargo')
depends=('arch-install-scripts' 'util-linux')

build() {
  cargo b -r
}

package() {
  cd ..
  # Create necessary directories
  install -d "${pkgdir}/usr/bin"

  # Install the compiled binary
  install -m755 "./target/release/${pkgname}" "${pkgdir}/usr/bin/"
}
