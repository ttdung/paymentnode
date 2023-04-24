package user

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/dungtt-astra/astra-go-sdk/account"
	"github.com/dungtt-astra/paymentnode/pkg/common"
	"time"
)

type User struct {
	Passcode    string
	Denom       string
	Deposit_Amt float64
	Account     *account.PrivateKeySerialized
	Nonce       uint64
}

func NewUser(passcode string, denom string, deposit_amt float64, mmemonic string) (*User, error) {

	acc, err := account.NewAccount(common.COINTYPE).ImportAccount(mmemonic)
	if err != nil {
		return nil, err
	}

	return &User{
		Passcode:    passcode,
		Denom:       denom,
		Deposit_Amt: deposit_amt,
		Account:     acc,
	}, nil
}

func (u *User) GetAccountAddr() string {
	return u.Account.AccAddress().String()
}

func (u *User) GetPubkey() cryptoTypes.PubKey {
	return u.Account.PublicKey()
}

func (u *User) GenerateHashcode(commitID string) (string, string) {

	now := time.Now()
	secret := fmt.Sprintf("%v:%v:%v", u.Passcode, now, commitID)

	hash := sha256.Sum256([]byte(secret))
	hashcode := base64.StdEncoding.EncodeToString(hash[:])

	return secret, hashcode
}
