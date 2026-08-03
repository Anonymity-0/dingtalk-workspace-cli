class DingtalkWorkspaceCliBeta < Formula
  desc "Automate DingTalk workspace tasks from the terminal (beta channel)"
  homepage "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli"
  version "1.0.56-beta.3"
  license "Apache-2.0"
  keg_only "it is the beta channel and conflicts with dingtalk-workspace-cli"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.3/dws-darwin-arm64.tar.gz"
      sha256 "5d35bb3fca7883a4ee51e1561aefd6b954b104313359a7c05b30a333ea41e749"
    else
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.3/dws-darwin-amd64.tar.gz"
      sha256 "5a94069a3ab2c811d915639bbfd9ff17035b77501d5f9070c0b540ffe525a41b"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.3/dws-linux-arm64.tar.gz"
      sha256 "e9cbc1c647f3ea703e1b93264c0e7d92871cfabe26fef26fbe17b9dff4b80c0f"
    else
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.3/dws-linux-amd64.tar.gz"
      sha256 "7c475841ea871d204f9f6efc7d6e5378cf19db4541aeaa8a27d1387fa6f2a1f1"
    end
  end

  resource "skills" do
    url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.3/dws-skills.zip"
    sha256 "b29b35345613bc9260ae37578eb982472591c9dc548b02176b5b9ba0bdc431f2"
  end

  def install
    root = Dir["dws-*"].find { |entry| File.directory?(entry) } || "."
    binary = File.join(root, "dws")
    raise "binary not found: #{binary}" unless File.exist?(binary)

    bin.install binary => "dws"

    %w[LICENSE NOTICE README.md CHANGELOG.md].each do |name|
      source = File.join(root, name)
      pkgshare.install source if File.exist?(source)
    end

    skill_dest = pkgshare/"skills/dws"
    skill_dest.mkpath
    resource("skills").stage do
      cp_r(Dir["*"], skill_dest)
    end
  end

  def caveats
    <<~EOS
      Agent Skills are bundled in #{pkgshare}/skills/dws.
      Run `dws skill setup` to install them into your Agent directories.
      This beta is keg-only. Add #{opt_bin} to PATH to use its `dws` binary.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/dws version")
  end
end
