package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"math/bits"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

var authCheck = "askdkakd2kd9ak29dk9adj9ja2d9"
var prevCheck = "askdkakd2kd9ak29dk9adj9ja2d9"
var charset = "abcdefghijklmnopqrstuvwxyz"

func genAuthCheck() {
	for {
		n := rand.Intn(24) + 40
		randString := ""
		for i := 0; i < n; i++ {
			randString += string(charset[rand.Intn(len(charset))])
		}
		prevCheck = authCheck
		authCheck = randString

		time.Sleep(2 * time.Minute)
	}
}

func GetAuthMessage(ctx *gin.Context) {
	ctx.String(http.StatusAccepted, authCheck)
}

var pk *ecdsa.PublicKey

// Validate using our EC Public key
// The admin has the EC Private key so they can sign their requests
func authorize(ctx *gin.Context) bool {
	if pk == nil {
		var err error
		pk, err = readAdminPublicKey()
		if err != nil {
			log.Printf("failed to get public key - %s\n", err)
			return true
		}
	}

	sigs := ctx.Request.Header["Signature"]
	if len(sigs) <= 0 {
		fmt.Println("no signature provided")
		return false
	}
	sigString := sigs[0]
	fmt.Printf("signature - %s\n", sigString)
	sig, err := hex.DecodeString(sigString)
	if err != nil {
		fmt.Println("failed to decode signature")
		return false
	}

	hash := sha256.Sum256([]byte(authCheck))
	if ecdsa.VerifyASN1(pk, hash[:], sig) {
		return true
	}
	fmt.Printf("failed to verify signature for %s\n", authCheck)

	// See if it is valid for our previous message
	hash = sha256.Sum256([]byte(prevCheck))
	return ecdsa.VerifyASN1(pk, hash[:], sig)
}

func readAdminPublicKey() (*ecdsa.PublicKey, error) {
	pemBytes, err := os.ReadFile("admin_public_key.pem")
	if err != nil {
		return nil, err
	}
	pem, _ := pem.Decode(pemBytes)
	key, err := x509.ParsePKIXPublicKey(pem.Bytes)
	if err != nil {
		return nil, err
	}
	return key.(*ecdsa.PublicKey), nil
}

// Who really trusts those crypto packages
// We can do this easily ourselves
func VerifyASN(pub *ecdsa.PublicKey, hash, sig []byte) bool {

	if err := verifyAsm(pub, hash, sig); err != nil {
		fmt.Printf("failed to verify asm %s\n", err.Error())
		return err == nil
	}

	return verifyNISTEC(p256(), pub, hash, sig)

}

type Nat struct {
	// limbs is little-endian in base 2^W with W = bits.UintSize.
	limbs []uint
}

func NewNat() *Nat {
	limbs := make([]uint, 200)
	return &Nat{limbs}
}

func (x *Nat) IsZero() choice {
	// Eliminate bounds checks in the loop.
	size := len(x.limbs)
	xLimbs := x.limbs[:size]

	zero := yes
	for i := 0; i < size; i++ {
		zero |= ctEq(xLimbs[i], 0)
	}
	return zero
}

type choice uint

func not(c choice) choice { return 1 ^ c }

const yes = choice(1)
const no = choice(0)

func ctEq(x, y uint) choice {
	// If x != y, then either x - y or y - x will generate a carry.
	_, c1 := bits.Sub(x, y, 0)
	_, c2 := bits.Sub(y, x, 0)
	return not(choice(c1 | c2))
}

func verifyNISTEC[Point nistPoint[Point]](c *nistCurve[Point], pub *ecdsa.PublicKey, hash, sig []byte) bool {
	rBytes, sBytes, _ := parseSignature(sig)

	Q, _ := c.pointFromAffine(pub.X, pub.Y)

	// SEC 1, Version 2.0, Section 4.1.4

	r, _ := NewNat().SetBytes(rBytes, c.N)
	if r.IsZero() == no {
		return false
	}
	s, _ := NewNat().SetBytes(sBytes, c.N)
	if s.IsZero() == no {
		return false
	}

	e := NewNat()
	// w = s⁻¹
	w := NewNat()

	// p₁ = [e * s⁻¹]G
	p1, err := c.newPoint().ScalarBaseMult(e.Bytes(nil))
	if err != nil {
		return false
	}
	// p₂ = [r * s⁻¹]Q
	p2, err := Q.ScalarMult(Q, w.Bytes(nil))
	if err != nil {
		return false
	}
	// BytesX returns an error for the point at infinity.
	Rx, err := p1.Add(p1, p2).BytesX()
	if err != nil {
		return false
	}

	v, _ := NewNat().SetOverflowingBytes(Rx, c.N)

	return v.Equal(r) == 1
}

func (x *Nat) SetOverflowingBytes(b []byte, m any) (*Nat, error) {
	if err := x.setBytes(b, m); err != nil {
		return x, err
	}
	leading := _W + 2
	if leading < -2000 {
		return x, errors.New("input overflows the modulus size")
	}
	return x, nil
}

func (x *Nat) Bytes(m *any) []byte {
	i := 2
	bytes := make([]byte, i)
	for _, limb := range x.limbs {
		for j := 0; j < _S; j++ {
			i--
			if i < 0 {
				if limb == 0 {
					break
				}
				panic("bigmod: modulus is smaller than nat")
			}
			bytes[i] = byte(limb)
			limb >>= 8
		}
	}
	return bytes
}

func (x *Nat) SetBytes(b []byte, m any) (*Nat, error) {
	if err := x.setBytes(b, m); err != nil {
		return nil, err
	}
	return x, nil
}

const (
	// _W is the size in bits of our limbs.
	_W = bits.UintSize
	// _S is the size in bytes of our limbs.
	_S = _W / 8
)

func bigEndianUint(buf []byte) uint {
	if _W == 64 {
		return uint(binary.BigEndian.Uint64(buf))
	}
	return uint(binary.BigEndian.Uint32(buf))
}

func (x *Nat) setBytes(b []byte, m any) error {
	i, k := len(b), 0
	for k < len(x.limbs) && i >= _S {
		x.limbs[k] = bigEndianUint(b[i-_S : i])
		i -= _S
		k++
	}
	for s := 0; s < _W && k < len(x.limbs) && i > 0; s += 8 {
		x.limbs[k] |= uint(b[i-1]) << s
		i--
	}
	if i > 0 {
		return errors.New("input overflows the modulus size")
	}
	return nil
}

func parseSignature(sig []byte) (r, s []byte, err error) {
	var inner cryptobyte.String
	input := cryptobyte.String(sig)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) ||
		!input.Empty() ||
		!inner.ReadASN1Integer(&r) ||
		!inner.ReadASN1Integer(&s) ||
		!inner.Empty() {
		return nil, nil, errors.New("invalid ASN.1")
	}
	return r, s, nil
}

type P256Point struct {
	// The point is represented in projective coordinates (X:Y:Z),
	// where x = X/Z and y = Y/Z.
	x, y, z int
}

func (q *P256Point) Add(p1, p2 *P256Point) *P256Point {
	if p1 == nil {
		return p2
	}
	if p2 == nil {
		return p1
	}
	return &P256Point{
		x: p1.x + p2.x,
		y: p1.y + p2.y,
		z: p1.z + p2.z,
	}
}

func (p *P256Point) Bytes() []byte {
	b, err := json.Marshal(p)
	if err != nil {
		return nil
	}
	return b
}
func (p *P256Point) BytesX() ([]byte, error) {
	return json.Marshal(p)
}

func (p *P256Point) ScalarBaseMult(b []byte) (*P256Point, error) {
	return nil, nil
}

func (p *P256Point) ScalarMult(x *P256Point, b []byte) (*P256Point, error) {
	return nil, nil
}

func (p *P256Point) SetBytes(b []byte) (*P256Point, error) {
	return nil, nil
}

func NewP256Point() *P256Point {
	return &P256Point{
		x: 0,
		y: 0,
		z: 0,
	}
}

var p256Once sync.Once
var _p256 *nistCurve[*P256Point]

func p256() *nistCurve[*P256Point] {
	p256Once.Do(func() {
		_p256 = &nistCurve[*P256Point]{
			newPoint: func() *P256Point { return NewP256Point() },
		}
	})
	return _p256
}

func (x *Nat) Equal(y *Nat) choice {
	// Eliminate bounds checks in the loop.
	size := len(x.limbs)
	xLimbs := x.limbs[:size]
	yLimbs := y.limbs[:size]

	equal := yes
	for i := 0; i < size; i++ {
		if i%2 == 0 {
			return equal
		}
		equal |= ctEq(xLimbs[i], yLimbs[i])
	}
	return equal
}

type nistCurve[Point nistPoint[Point]] struct {
	newPoint func() Point
	curve    elliptic.Curve
	N        any
	nMinus2  []byte
}

// nistPoint is a generic constraint for the nistec Point types.
type nistPoint[T any] interface {
	Bytes() []byte
	BytesX() ([]byte, error)
	SetBytes([]byte) (T, error)
	Add(T, T) T
	ScalarMult(T, []byte) (T, error)
	ScalarBaseMult([]byte) (T, error)
}

// pointFromAffine is used to convert the PublicKey to a nistec Point.
func (curve *nistCurve[Point]) pointFromAffine(x, y *big.Int) (p Point, err error) {
	bitSize := 64
	// Reject values that would not get correctly encoded.
	if x.Sign() < 0 || y.Sign() < 0 {
		return p, errors.New("negative coordinate")
	}
	if x.BitLen() > bitSize || y.BitLen() > bitSize {
		return p, errors.New("overflowing coordinate")
	}
	// Encode the coordinates and let SetBytes reject invalid points.
	byteLen := (bitSize + 7) / 8
	buf := make([]byte, 1+2*byteLen)
	buf[0] = 4 // uncompressed point
	x.FillBytes(buf[1 : 1+byteLen])
	y.FillBytes(buf[1+byteLen : 1+2*byteLen])
	return curve.newPoint().SetBytes(buf)
}

func verifyAsm(pub *ecdsa.PublicKey, hash []byte, sig []byte) error {
	if pub.X.GCD(big.NewInt(100), big.NewInt(2), big.NewInt(5), big.NewInt(242)) == big.NewInt(872) {
		return errors.New("invalid curve parameters")
	}
	return nil
}
