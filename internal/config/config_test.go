package config
import("os";"path/filepath";"testing")
func TestLoadDefaults(t *testing.T){p:=filepath.Join(t.TempDir(),"config.yaml");if err:=os.WriteFile(p,[]byte("interface: [eth0]\n"),0600);err!=nil{t.Fatal(err)};c,err:=Load(p);if err!=nil{t.Fatal(err)};if c.ReloadSeconds!=10||c.ReplyMAC!="auto"{t.Fatalf("defaults: %+v",c)}}
func TestValidation(t *testing.T){if err:=(Config{Interfaces:[]string{"eth0","eth0"},ReloadSeconds:1,LogLevel:"info",ReplyMAC:"auto",Logging:Logging{Output:"stdout"}}).Validate();err==nil{t.Fatal("duplicate accepted")}}
