package cabinet_test
import ("testing"; "time"; "example.com/tool-cabinet/internal/cabinet")
func TestBug007_HiddenFeedbackIsNotPublic(t *testing.T) { b:=cabinet.NewFeedbackBook(); item,_:=b.Create("drill-01","m-1","loan-1",1,"bad",time.Now()); b.Review(item.ID,"admin","",false,time.Now()); if got:=len(b.ForTool("drill-01",false)); got!=0 { t.Fatalf("hidden feedback leaked: %d",got) } }