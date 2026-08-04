class DingtalkWorkspaceCliBeta < Formula
  desc "Automate DingTalk workspace tasks from the terminal (beta channel)"
  homepage "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli"
  version "1.0.56-beta.4"
  license "Apache-2.0"
  keg_only "it is the beta channel and conflicts with dingtalk-workspace-cli"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.4/dws-darwin-arm64.tar.gz"
      sha256 "f1f9b6394137edbd0b08d632aab34e92a0f3f81d80107a47de1bec9b384f0515"
    else
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.4/dws-darwin-amd64.tar.gz"
      sha256 "cd3c64d20723c420e2490405d0bf8eecfd7e2b8fc352f63f23de5847a1d38f55"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.4/dws-linux-arm64.tar.gz"
      sha256 "910918d88074534e680a2e320d3cb364ad092e96b9c422f9e75d11c9c0815dd8"
    else
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.4/dws-linux-amd64.tar.gz"
      sha256 "172fe0d84443be953d0c6f2c2433540e4b972fbe7776cff1417ec9c73723552b"
    end
  end

  resource "skills" do
    url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.4/dws-skills.zip"
    sha256 "a3457befe858cbf3fe85848428b630bfd3a5f626256ed6b49415267948915152"
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
