package cash_test

import (
	"be-dikasa/internal/cash"
	"be-dikasa/internal/catalog"
	"be-dikasa/internal/payment"
	"be-dikasa/internal/refund"
	"be-dikasa/internal/shared/imagefile"
	"be-dikasa/internal/shared/supabasestorage"
	"be-dikasa/internal/user"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"
)

type integrationProofObject struct {
	contentType string
	content     []byte
}

type integrationProofBucket struct {
	mu      sync.Mutex
	objects map[string]integrationProofObject
}

func newIntegrationProofStore(t *testing.T) (*cash.SupabaseMovementProofStore, *integrationProofBucket) {
	t.Helper()
	bucket := &integrationProofBucket{objects: map[string]integrationProofObject{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/storage/v1/object/dikasa-images/")
		bucket.mu.Lock()
		defer bucket.mu.Unlock()
		if request.Method == http.MethodDelete {
			delete(bucket.objects, name)
			writer.WriteHeader(http.StatusOK)
			return
		}
		content, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		bucket.objects[name] = integrationProofObject{contentType: request.Header.Get("Content-Type"), content: content}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	uploader, err := supabasestorage.NewUploader(server.URL, "dikasa-images", "secret", supabasestorage.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return cash.NewSupabaseMovementProofStore(uploader), bucket
}

func TestMovementPostgresConcurrentOutflowReplayAndClose(t *testing.T) {
	db := closingDB(t)
	ctx := context.Background()
	var a user.User
	db.Preload("Role").Where("username='admin'").First(&a)
	sid := uuid.NewString()
	if e := db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status) VALUES(?,?,?,100000,'open')", sid, *a.OutletID, a.ID).Error; e != nil {
		t.Fatal(e)
	}
	s := cash.NewMovementService(cash.NewMovementRepository(db), user.NewGORMRepository(db), nil, nil)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Create(ctx, a.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "out", Amount: 80000, Name: "Deposit", Reason: "Bank deposit"})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, cash.ErrMovementInsufficient) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatal("double spent drawer", wins)
	}
	c := cash.MovementCommand{RequestID: uuid.NewString(), Kind: "in", Amount: 20000, Name: "Float", Reason: "Extra change"}
	m, e := s.Create(ctx, a.ID, sid, c)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Create(ctx, a.ID, sid, c)
	if e != nil || again.ID != m.ID {
		t.Fatal("replay", e)
	}
	c.Amount++
	if _, e = s.Create(ctx, a.ID, sid, c); !errors.Is(e, cash.ErrMovementRequestUsed) {
		t.Fatal("changed request accepted", e)
	}
	reconciliation := cash.NewReconciliationService(cash.NewReconciliationRepository(db), user.NewGORMRepository(db))
	p, e := reconciliation.Preview(ctx, a.ID, sid, cash.CloseSelection{CountedCash: 40000})
	if e != nil {
		t.Fatal(e)
	}
	rev, e := s.Reverse(ctx, a.ID, sid, m.ID, cash.ReversalCommand{RequestID: uuid.NewString(), Reason: "Wrong float"})
	if e != nil || rev.Amount != 20000 || rev.Kind != "out" {
		t.Fatal("reversal", e)
	}
	if _, e = s.Reverse(ctx, a.ID, sid, m.ID, cash.ReversalCommand{RequestID: uuid.NewString(), Reason: "Again"}); !errors.Is(e, cash.ErrMovementReversed) {
		t.Fatal("second reversal", e)
	}
	_, e = reconciliation.Close(ctx, a.ID, sid, cash.CloseCommand{RequestID: uuid.NewString(), PreviewHash: p.PreviewHash, CloseSelection: cash.CloseSelection{CountedCash: 40000}})
	if !errors.Is(e, cash.ErrCloseStale) {
		t.Fatal("preview did not stale", e)
	}
	p, e = reconciliation.Preview(ctx, a.ID, sid, cash.CloseSelection{CountedCash: 20000})
	if e != nil {
		t.Fatal(e)
	}
	_, e = reconciliation.Close(ctx, a.ID, sid, cash.CloseCommand{RequestID: uuid.NewString(), PreviewHash: p.PreviewHash, CloseSelection: cash.CloseSelection{CountedCash: 20000}})
	if e != nil {
		t.Fatal(e)
	}
	c.Amount = 20000
	again, e = s.Create(ctx, a.ID, sid, c)
	if e != nil || again.ID != m.ID {
		t.Fatal("closed replay", e)
	}
	if _, e = s.Create(ctx, a.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "in", Amount: 1, Name: "New", Reason: "Closed"}); !errors.Is(e, cash.ErrAlreadyClosed) {
		t.Fatal("closed write accepted", e)
	}
	history, e := s.Activity(ctx, a.ID, sid, 1, 20)
	if e != nil || history.Totals == nil || history.Totals.ExpectedCash != 20000 || history.Meta.Total != 4 {
		t.Fatal("activity", history, e)
	}
}

func TestMovementPrivateProofAndActorPermissions(t *testing.T) {
	db := closingDB(t)
	ctx := context.Background()
	var a user.User
	db.Preload("Role").Where("username='admin'").First(&a)
	sid := uuid.NewString()
	db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status) VALUES(?,?,?,100000,'open')", sid, *a.OutletID, a.ID)
	store, _ := newIntegrationProofStore(t)
	svc := cash.NewMovementService(cash.NewMovementRepository(db), user.NewGORMRepository(db), store, nil)
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	v, e := imagefile.Validate(imagefile.Upload{Reader: bytes.NewReader(b.Bytes()), Size: int64(b.Len()), Filename: "evidence.png", ContentType: "image/png"}, imagefile.StandardPolicy)
	if e != nil {
		t.Fatal(e)
	}
	m, e := svc.Create(ctx, a.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "in", Amount: 100, Name: "Float", Reason: "Additional change", Proof: &v})
	if e != nil {
		t.Fatal(e)
	}
	router, api := humatest.New(t)
	cash.RegisterMovementRoutes(api, cash.NewMovementHandler(svc))
	path := "/cash-sessions/" + sid + "/movements/" + m.ID + "/proof"
	if w := api.DoCtx(ctx, "GET", path); w.Code != 401 {
		t.Fatal("unauthorized proof", w.Code)
	}
	authctx := user.WithAuthenticatedUserID(ctx, a.ID)
	// Real multipart create/replay with proof, rather than only service-level fixtures.
	requestID := uuid.NewString()
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	for key, value := range map[string]string{"request_id": requestID, "kind": "in", "amount": "250", "name": "HTTP Float", "reason": "Actual multipart contract"} {
		if e := writer.WriteField(key, value); e != nil {
			t.Fatal(e)
		}
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="proof"; filename="evidence.png"`)
	header.Set("Content-Type", "image/png")
	part, e := writer.CreatePart(header)
	if e != nil {
		t.Fatal(e)
	}
	part.Write(b.Bytes())
	writer.Close()
	var createdID string
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/cash-sessions/"+sid+"/movements", bytes.NewReader(form.Bytes())).WithContext(authctx)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		reply := httptest.NewRecorder()
		router.ServeHTTP(reply, req)
		if reply.Code != 201 {
			t.Fatal("multipart create/replay", reply.Code, reply.Body.String())
		}
		var data struct {
			Data cash.MovementRecord `json:"data"`
		}
		if e = json.Unmarshal(reply.Body.Bytes(), &data); e != nil || !data.Data.HasProof {
			t.Fatal("multipart proof absent", e)
		}
		if i == 0 {
			createdID = data.Data.ID
		} else if data.Data.ID != createdID {
			t.Fatal("HTTP replay inserted again")
		}
	}

	w := api.DoCtx(authctx, "GET", path)
	var proofResponse struct {
		Data cash.ProofLink `json:"data"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &proofResponse); w.Code != 200 || e != nil || !strings.Contains(proofResponse.Data.URL, "/storage/v1/object/public/dikasa-images/") {
		t.Fatal("Supabase proof URL", w.Code, w.Body.String(), e)
	}
	var role user.Role
	db.Where("name='cashier'").First(&role)
	other := a
	other.ID = uuid.NewString()
	other.Username = "cashier2"
	other.PhoneNumber = "08123459999"
	other.RoleID = role.ID
	other.Role = user.Role{}
	other.Outlet = nil
	other.Gender = nil
	other.Email = nil
	if e = db.Create(&other).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Create(ctx, other.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "in", Amount: 1, Name: "No", Reason: "No"}); !errors.Is(e, cash.ErrCloseForbidden) {
		t.Fatal("non opener write", e)
	}
	if activity, readErr := svc.Activity(ctx, other.ID, sid, 1, 20); readErr != nil || activity.Capabilities.CanCreate || activity.Capabilities.CanUploadProof {
		t.Fatal("same outlet read denied", e)
	}
	db.Model(&user.User{}).Where("id=?", other.ID).Update("is_active", false)
	if _, e = svc.Proof(ctx, other.ID, sid, m.ID); !errors.Is(e, cash.ErrCloseForbidden) {
		t.Fatal("inactive proof", e)
	}
	// Actor/metadata forged by an HTTP caller are never used as account authority.
	db.Model(&user.User{}).Where("id=?", other.ID).Update("is_active", true)
	if _, e = svc.ByRequest(ctx, other.ID, sid, m.RequestID); !errors.Is(e, cash.ErrNotFound) {
		t.Fatal("another actor recovered command", e)
	}
	// A movement cannot target a foreign outlet even with a known movement/session UUID.
	out := user.Outlet{ID: uuid.NewString(), Name: "Other", BusinessType: "retail", Address: "Other", PhoneNumber: "0800000000", IsActive: true}
	if e = db.Create(&out).Error; e != nil {
		t.Fatal(e)
	}
	db.Model(&user.User{}).Where("id=?", other.ID).Update("outlet_id", out.ID)
	if _, e = svc.Proof(ctx, other.ID, sid, m.ID); !errors.Is(e, cash.ErrNotFound) {
		t.Fatal("foreign proof", e)
	}
}

func TestMovementRaceWithCloseAndExactReplay(t *testing.T) {
	db := closingDB(t)
	ctx := context.Background()
	var a user.User
	db.Preload("Role").Where("username='admin'").First(&a)
	sid := uuid.NewString()
	db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status) VALUES(?,?,?,100000,'open')", sid, *a.OutletID, a.ID)
	svc := cash.NewMovementService(cash.NewMovementRepository(db), user.NewGORMRepository(db), nil, nil)
	closeSvc := cash.NewReconciliationService(cash.NewReconciliationRepository(db), user.NewGORMRepository(db))
	c := cash.MovementCommand{RequestID: uuid.NewString(), Kind: "in", Amount: 100, Name: "Float", Reason: "Change"}
	results := make(chan cash.MovementRecord, 2)
	errs := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() { <-start; m, e := svc.Create(ctx, a.ID, sid, c); results <- m; errs <- e }()
	}
	close(start)
	m1, m2 := <-results, <-results
	if e := <-errs; e != nil {
		t.Fatal(e)
	}
	if e := <-errs; e != nil || m1.ID != m2.ID {
		t.Fatal("concurrent replay", e)
	}
	p, e := closeSvc.Preview(ctx, a.ID, sid, cash.CloseSelection{CountedCash: 100100})
	if e != nil {
		t.Fatal(e)
	}
	start = make(chan struct{})
	moved := make(chan error, 1)
	closed := make(chan error, 1)
	go func() {
		<-start
		_, e := svc.Create(ctx, a.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "out", Amount: 10, Name: "Deposit", Reason: "Bank"})
		moved <- e
	}()
	go func() {
		<-start
		_, e := closeSvc.Close(ctx, a.ID, sid, cash.CloseCommand{RequestID: uuid.NewString(), PreviewHash: p.PreviewHash, CloseSelection: cash.CloseSelection{CountedCash: 100100}})
		closed <- e
	}()
	close(start)
	me, ce := <-moved, <-closed
	if !((me == nil && errors.Is(ce, cash.ErrCloseStale)) || (errors.Is(me, cash.ErrAlreadyClosed) && ce == nil)) {
		t.Fatal("movement/close not serialized", me, ce)
	}
}

func TestMovementRollbackRemovesUnreferencedProof(t *testing.T) {
	db := closingDB(t)
	ctx := context.Background()
	var a user.User
	db.Preload("Role").Where("username='admin'").First(&a)
	sid := uuid.NewString()
	db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status) VALUES(?,?,?,100000,'open')", sid, *a.OutletID, a.ID)
	store, bucket := newIntegrationProofStore(t)
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	v, e := imagefile.Validate(imagefile.Upload{Reader: bytes.NewReader(b.Bytes()), Size: int64(b.Len()), Filename: "p.png", ContentType: "image/png"}, imagefile.StandardPolicy)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Exec(`CREATE FUNCTION reject_cash_movement() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$; CREATE TRIGGER reject_movement BEFORE INSERT ON cash_movements FOR EACH ROW EXECUTE FUNCTION reject_cash_movement();`).Error; e != nil {
		t.Fatal(e)
	}
	svc := cash.NewMovementService(cash.NewMovementRepository(db), user.NewGORMRepository(db), store, nil)
	_, e = svc.Create(ctx, a.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "in", Amount: 10, Name: "Float", Reason: "Failure injection", Proof: &v})
	if e == nil {
		t.Fatal("write failure hidden")
	}
	var count int64
	db.Model(&cash.MovementRecord{}).Count(&count)
	bucket.mu.Lock()
	objectCount := len(bucket.objects)
	bucket.mu.Unlock()
	if count != 0 || objectCount != 0 {
		t.Fatal("rollback leaked row or proof", count, objectCount)
	}
}

func TestMovementWaitsForRealSaleAndRefundCommit(t *testing.T) {
	db := closingDB(t)
	ctx := context.Background()
	var a user.User
	db.Preload("Role").Where("username='admin'").First(&a)
	sid := uuid.NewString()
	db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status) VALUES(?,?,?,0,'open')", sid, *a.OutletID, a.ID)
	var product catalog.Product
	db.Where("outlet_id=?", *a.OutletID).First(&product)
	var orderType catalog.OrderType
	db.Where("outlet_id=?", *a.OutletID).First(&orderType)
	svc := cash.NewMovementService(cash.NewMovementRepository(db), user.NewGORMRepository(db), nil, nil)
	tx := db.Begin()
	defer tx.Rollback()
	pay := payment.NewService(payment.NewGORMRepository(tx), user.NewGORMRepository(tx))
	sale, e := pay.CreateTransaction(ctx, a.ID, payment.TransactionMutation{OrderTypeID: orderType.ID, PaymentMethod: "cash", Subtotal: product.Price, TotalAmount: product.Price, PaidAmount: product.Price, Items: []payment.TransactionItemMutation{{ProductID: product.ID, ProductName: product.Name, UnitPrice: product.Price, Quantity: 1, LineTotal: product.Price}}})
	if e != nil {
		t.Fatal(e)
	}
	wait := make(chan error, 1)
	go func() {
		_, e := svc.Create(ctx, a.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "out", Amount: product.Price, Name: "Deposit", Reason: "Wait for cash sale"})
		wait <- e
	}()
	select {
	case e := <-wait:
		t.Fatal("movement bypassed uncommitted sale", e)
	case <-time.After(150 * time.Millisecond):
	}
	if e = tx.Commit().Error; e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-wait:
		if e != nil {
			t.Fatal("committed cash omitted", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("movement did not resume")
	}
	if _, e = svc.Create(ctx, a.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "in", Amount: product.Price, Name: "Float", Reason: "Fund refund"}); e != nil {
		t.Fatal(e)
	}
	rtx := db.Begin()
	defer rtx.Rollback()
	refunds := refund.NewService(refund.NewGORMRepository(rtx), user.NewGORMRepository(rtx))
	detail, e := refunds.Detail(ctx, a.ID, sale.ID)
	if e != nil {
		t.Fatal(e)
	}
	selection := refund.Selection{Mode: "full", Method: "cash", Reason: "Return", Items: []refund.SelectedItem{{TransactionItemID: detail.Items[0].ID, Quantity: 1}}}
	preview, e := refunds.Preview(ctx, a.ID, sale.ID, selection)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = refunds.Confirm(ctx, a.ID, sale.ID, refund.Command{Selection: selection, RequestID: uuid.NewString(), PreviewHash: preview.SelectionHash}); e != nil {
		t.Fatal(e)
	}
	go func() {
		_, e := svc.Create(ctx, a.ID, sid, cash.MovementCommand{RequestID: uuid.NewString(), Kind: "out", Amount: 1, Name: "After refund", Reason: "Must see payout"})
		wait <- e
	}()
	select {
	case e := <-wait:
		t.Fatal("movement bypassed uncommitted refund", e)
	case <-time.After(150 * time.Millisecond):
	}
	if e = rtx.Commit().Error; e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-wait:
		if !errors.Is(e, cash.ErrMovementInsufficient) {
			t.Fatal("refund payout omitted", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("movement did not resume after refund")
	}
}
