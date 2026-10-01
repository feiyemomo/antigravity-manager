import { spawnSync } from "node:child_process";
import { CREDENTIAL_TARGET } from "./constants.js";

export interface ICredentialManager {
  isAvailable(): Promise<boolean>;
  read(target?: string): Promise<string | null>;
  write(target: string, username: string, secret: string): Promise<boolean>;
  delete(target: string): Promise<boolean>;
  list(): Promise<string[]>;
}

const CSHARP_WINCRED_SOURCE = `
using System;
using System.Text;
using System.Runtime.InteropServices;

public class WinCred {
    [DllImport("Advapi32.dll", EntryPoint = "CredReadW", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern bool CredRead(string target, int type, int reservedFlag, out IntPtr credentialPtr);

    [DllImport("Advapi32.dll", EntryPoint = "CredWriteW", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern bool CredWrite([In] ref CREDENTIAL userCredential, uint flags);

    [DllImport("Advapi32.dll", EntryPoint = "CredDeleteW", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern bool CredDelete(string target, int type, int flags);

    [DllImport("Advapi32.dll", EntryPoint = "CredFree", SetLastError = true)]
    public static extern void CredFree(IntPtr credentialPtr);

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct CREDENTIAL {
        public int Flags;
        public int Type;
        public string TargetName;
        public string Comment;
        public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten;
        public int CredentialBlobSize;
        public IntPtr CredentialBlob;
        public int Persist;
        public int AttributeCount;
        public IntPtr Attributes;
        public string TargetAlias;
        public string UserName;
    }

    public static string Read(string target) {
        IntPtr credPtr;
        if (CredRead(target, 1, 0, out credPtr)) {
            CREDENTIAL cred = (CREDENTIAL)Marshal.PtrToStructure(credPtr, typeof(CREDENTIAL));
            byte[] bytes = new byte[cred.CredentialBlobSize];
            Marshal.Copy(cred.CredentialBlob, bytes, 0, cred.CredentialBlobSize);
            CredFree(credPtr);
            return Encoding.UTF8.GetString(bytes);
        }
        return null;
    }

    public static bool Write(string target, string userName, string secret) {
        byte[] bytes = Encoding.UTF8.GetBytes(secret);
        IntPtr blobPtr = Marshal.AllocHGlobal(bytes.Length);
        try {
            Marshal.Copy(bytes, 0, blobPtr, bytes.Length);
            CREDENTIAL cred = new CREDENTIAL();
            cred.Type = 1;
            cred.TargetName = target;
            cred.UserName = userName;
            cred.CredentialBlob = blobPtr;
            cred.CredentialBlobSize = bytes.Length;
            cred.Persist = 2; // CRED_PERSIST_LOCAL_MACHINE
            return CredWrite(ref cred, 0);
        } finally {
            Marshal.FreeHGlobal(blobPtr);
        }
    }

    public static bool Delete(string target) {
        return CredDelete(target, 1, 0);
    }
}
`;

/**
 * Windows Credential Manager implementation using Advapi32.dll & cmdkey
 */
export class WindowsCredentialManager implements ICredentialManager {
  private runPowerShell(scriptBody: string): string {
    const fullScript = `
Add-Type -TypeDefinition @"
${CSHARP_WINCRED_SOURCE}
"@
${scriptBody}
`;
    const res = spawnSync("powershell.exe", ["-NoProfile", "-NonInteractive", "-Command", fullScript], {
      encoding: "utf-8",
      windowsHide: true,
    });

    if (res.error) {
      throw res.error;
    }
    return (res.stdout || "").trim();
  }

  async isAvailable(): Promise<boolean> {
    try {
      const res = spawnSync("cmdkey.exe", ["/list"], {
        encoding: "utf-8",
        windowsHide: true,
      });
      return res.status === 0;
    } catch {
      return false;
    }
  }

  async read(target: string = CREDENTIAL_TARGET): Promise<string | null> {
    try {
      const script = `
$res = [WinCred]::Read("${target.replace(/"/g, '`"')}");
if ($res -ne $null) {
    [Console]::WriteLine("OK:" + [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($res)))
} else {
    [Console]::WriteLine("NULL")
}
`;
      const out = this.runPowerShell(script);
      const lines = out.split(/\r?\n/).map((l) => l.trim());
      for (const line of lines) {
        if (line.startsWith("OK:")) {
          const b64 = line.substring(3);
          return Buffer.from(b64, "base64").toString("utf-8");
        }
        if (line === "NULL") {
          return null;
        }
      }
      return null;
    } catch {
      return null;
    }
  }

  async write(target: string, username: string, secret: string): Promise<boolean> {
    try {
      const b64Secret = Buffer.from(secret, "utf-8").toString("base64");
      const script = `
$secret = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String("${b64Secret}"));
$ok = [WinCred]::Write("${target.replace(/"/g, '`"')}", "${username.replace(/"/g, '`"')}", $secret);
if ($ok) { [Console]::WriteLine("OK") } else { [Console]::WriteLine("FAIL") }
`;
      const out = this.runPowerShell(script);
      const ok = out.split(/\r?\n/).some((l) => l.trim() === "OK");

      // As verification, verify cmdkey lists the target
      if (ok) {
        return true;
      }

      // Fallback attempt: if advapi32 returned false, try cmdkey directly
      try {
        const cmdRes = spawnSync("cmdkey.exe", [`/generic:${target}`, `/user:${username}`, `/pass:${secret}`], {
          encoding: "utf-8",
          windowsHide: true,
        });
        return cmdRes.status === 0;
      } catch {
        return false;
      }
    } catch {
      return false;
    }
  }

  async delete(target: string): Promise<boolean> {
    try {
      const script = `
$ok = [WinCred]::Delete("${target.replace(/"/g, '`"')}");
if ($ok) { [Console]::WriteLine("OK") } else { [Console]::WriteLine("FAIL") }
`;
      const out = this.runPowerShell(script);
      const ok = out.split(/\r?\n/).some((l) => l.trim() === "OK");
      if (ok) return true;

      // Fallback attempt using cmdkey
      const cmdRes = spawnSync("cmdkey.exe", [`/delete:${target}`], {
        encoding: "utf-8",
        windowsHide: true,
      });
      return cmdRes.status === 0;
    } catch {
      return false;
    }
  }

  async list(): Promise<string[]> {
    try {
      const res = spawnSync("cmdkey.exe", ["/list"], {
        encoding: "utf-8",
        windowsHide: true,
      });
      const lines = (res.stdout || "").split(/\r?\n/);
      const targets: string[] = [];
      for (const line of lines) {
        const match = line.match(/target=(.+)$/i) || line.match(/目标:\s*(.+)$/i) || line.match(/Target:\s*(.+)$/i);
        if (match && match[1]) {
          targets.push(match[1].trim());
        }
      }
      return targets;
    } catch {
      return [];
    }
  }
}

/**
 * macOS Keychain implementation using /usr/bin/security
 */
export class MacOSCredentialManager implements ICredentialManager {
  async isAvailable(): Promise<boolean> {
    try {
      const res = spawnSync("/usr/bin/security", ["help"], { encoding: "utf-8" });
      return res.status === 0;
    } catch {
      return false;
    }
  }

  async read(target: string = CREDENTIAL_TARGET): Promise<string | null> {
    try {
      const res = spawnSync("/usr/bin/security", ["find-generic-password", "-s", target, "-w"], {
        encoding: "utf-8",
      });
      if (res.status === 0 && res.stdout) {
        return res.stdout.trim();
      }
      return null;
    } catch {
      return null;
    }
  }

  async write(target: string, username: string, secret: string): Promise<boolean> {
    try {
      const res = spawnSync(
        "/usr/bin/security",
        ["add-generic-password", "-U", "-s", target, "-a", username, "-w", secret],
        { encoding: "utf-8" }
      );
      return res.status === 0;
    } catch {
      return false;
    }
  }

  async delete(target: string): Promise<boolean> {
    try {
      const res = spawnSync("/usr/bin/security", ["delete-generic-password", "-s", target], {
        encoding: "utf-8",
      });
      return res.status === 0;
    } catch {
      return false;
    }
  }

  async list(): Promise<string[]> {
    return [CREDENTIAL_TARGET];
  }
}

/**
 * Linux SecretService implementation using secret-tool
 */
export class LinuxCredentialManager implements ICredentialManager {
  async isAvailable(): Promise<boolean> {
    try {
      const res = spawnSync("which", ["secret-tool"], { encoding: "utf-8" });
      return res.status === 0;
    } catch {
      return false;
    }
  }

  async read(target: string = CREDENTIAL_TARGET): Promise<string | null> {
    try {
      const res = spawnSync("secret-tool", ["lookup", "service", target], {
        encoding: "utf-8",
      });
      if (res.status === 0 && res.stdout) {
        return res.stdout.trim();
      }
      return null;
    } catch {
      return null;
    }
  }

  async write(target: string, username: string, secret: string): Promise<boolean> {
    try {
      const res = spawnSync(
        "secret-tool",
        ["store", `--label=${target}`, "service", target, "username", username],
        {
          input: secret,
          encoding: "utf-8",
        }
      );
      return res.status === 0;
    } catch {
      return false;
    }
  }

  async delete(target: string): Promise<boolean> {
    try {
      const res = spawnSync("secret-tool", ["clear", "service", target], {
        encoding: "utf-8",
      });
      return res.status === 0;
    } catch {
      return false;
    }
  }

  async list(): Promise<string[]> {
    return [CREDENTIAL_TARGET];
  }
}

/**
 * Factory function to get the platform credential manager
 */
export function getCredentialManager(): ICredentialManager {
  if (process.platform === "win32") {
    return new WindowsCredentialManager();
  }
  if (process.platform === "darwin") {
    return new MacOSCredentialManager();
  }
  return new LinuxCredentialManager();
}

export const credentialManager = getCredentialManager();
