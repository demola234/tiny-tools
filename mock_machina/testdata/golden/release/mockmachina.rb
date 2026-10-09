class Mockmachina < Formula
  desc "Mock HTTP APIs from contract files in your repository"
  homepage "https://github.com/demola234/tiny-tools/tree/main/mock_machina"
  version "1.2.3"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/demola234/tiny-tools/releases/download/mock_machina/v1.2.3/mockmachina_1.2.3_darwin_arm64.tar.gz"
      sha256 "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    end
    on_intel do
      url "https://github.com/demola234/tiny-tools/releases/download/mock_machina/v1.2.3/mockmachina_1.2.3_darwin_amd64.tar.gz"
      sha256 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/demola234/tiny-tools/releases/download/mock_machina/v1.2.3/mockmachina_1.2.3_linux_arm64.tar.gz"
      sha256 "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    end
    on_intel do
      url "https://github.com/demola234/tiny-tools/releases/download/mock_machina/v1.2.3/mockmachina_1.2.3_linux_amd64.tar.gz"
      sha256 "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    end
  end

  def install
    bin.install "mockmachina"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/mockmachina --version")
  end
end
