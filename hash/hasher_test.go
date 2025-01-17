// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package hash_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/kratos/hash"
	"github.com/ory/kratos/internal"
)

func mkpw(t *testing.T, length int) []byte {
	pw := make([]byte, length)
	_, err := rand.Read(pw)
	require.NoError(t, err)
	return pw
}

func TestArgonHasher(t *testing.T) {
	t.Parallel()
	for _, pwLength := range []int{
		8,
		16,
		32,
		64,
		128,
	} {
		pwLength := pwLength
		t.Run(fmt.Sprintf("length=%dchars", pwLength), func(t *testing.T) {
			t.Parallel()
			pw := mkpw(t, pwLength)
			_, reg := internal.NewFastRegistryWithMocks(t)
			h := hash.NewHasherArgon2(reg)
			hs, err := h.Generate(context.Background(), pw)
			require.NoError(t, err)
			assert.NotEqual(t, pw, hs)

			t.Logf("hash: %s", hs)
			require.NoError(t, hash.CompareArgon2id(context.Background(), pw, hs))

			mod := make([]byte, len(pw))
			copy(mod, pw)
			mod[len(pw)-1] = ^pw[len(pw)-1]
			require.Error(t, hash.CompareArgon2id(context.Background(), mod, hs))
		})
	}
}

func TestBcryptHasherGeneratesErrorWhenPasswordIsLong(t *testing.T) {
	t.Parallel()
	_, reg := internal.NewFastRegistryWithMocks(t)
	hasher := hash.NewHasherBcrypt(reg)

	password := mkpw(t, 73)
	res, err := hasher.Generate(context.Background(), password)

	assert.Error(t, err, "password is too long")
	assert.Nil(t, res)
}

func TestBcryptHasherGeneratesHash(t *testing.T) {
	t.Parallel()
	for _, pwLength := range []int{
		8,
		16,
		32,
		64,
		72,
	} {
		pwLength := pwLength
		t.Run(fmt.Sprintf("length=%dchars", pwLength), func(t *testing.T) {
			t.Parallel()
			_, reg := internal.NewFastRegistryWithMocks(t)
			hasher := hash.NewHasherBcrypt(reg)
			pw := mkpw(t, pwLength)
			hs, err := hasher.Generate(context.Background(), pw)

			assert.Nil(t, err)
			assert.True(t, hasher.Understands(hs))

			// Valid format: $2a$12$[22 character salt][31 character hash]
			assert.Equal(t, 60, len(string(hs)), "invalid bcrypt hash length")
			assert.Equal(t, "$2a$04$", string(hs)[:7], "invalid bcrypt identifier")
		})
	}
}

func TestComparatorBcryptFailsWhenPasswordIsTooLong(t *testing.T) {
	t.Parallel()
	password := mkpw(t, 73)
	err := hash.CompareBcrypt(context.Background(), password, []byte("hash"))

	assert.Error(t, err, "password is too long")
}

func TestComparatorBcryptSuccess(t *testing.T) {
	t.Parallel()
	for _, pwLength := range []int{
		8,
		16,
		32,
		64,
		72,
	} {
		pwLength := pwLength
		t.Run(fmt.Sprintf("length=%dchars", pwLength), func(t *testing.T) {
			t.Parallel()
			_, reg := internal.NewFastRegistryWithMocks(t)
			hasher := hash.NewHasherBcrypt(reg)
			pw := mkpw(t, pwLength)

			hs, err := hasher.Generate(context.Background(), pw)

			assert.Nil(t, err)
			assert.True(t, hasher.Understands(hs))

			err = hash.CompareBcrypt(context.Background(), pw, hs)
			assert.Nil(t, err, "hash validation fails")
		})
	}
}

func TestComparatorBcryptFail(t *testing.T) {
	t.Parallel()
	for _, pwLength := range []int{
		8,
		16,
		32,
		64,
		72,
	} {
		pwLength := pwLength
		t.Run(fmt.Sprintf("length=%dchars", pwLength), func(t *testing.T) {
			t.Parallel()
			pw := mkpw(t, pwLength)
			mod := make([]byte, len(pw))
			copy(mod, pw)
			mod[len(pw)-1] = ^pw[len(pw)-1]

			err := hash.CompareBcrypt(context.Background(), pw, mod)
			assert.Error(t, err)
		})
	}
}

func TestPbkdf2Hasher(t *testing.T) {
	t.Parallel()
	for _, pwLength := range []int{
		8,
		16,
		32,
		64,
		128,
	} {
		pwLength := pwLength
		t.Run(fmt.Sprintf("length=%dchars", pwLength), func(t *testing.T) {
			t.Parallel()
			for _, algorithm := range []string{
				"sha1",
				"sha224",
				"sha256",
				"sha384",
				"sha512",
			} {
				algorithm := algorithm
				t.Run(fmt.Sprintf("algorithm=%s", algorithm), func(t *testing.T) {
					t.Parallel()
					hasher := &hash.Pbkdf2{
						Algorithm:  algorithm,
						Iterations: 100_000,
						SaltLength: 32,
						KeyLength:  32,
					}
					pw := mkpw(t, pwLength)
					t.Logf("%d", pwLength)
					hs, err := hasher.Generate(context.Background(), pw)
					require.NoError(t, err)
					assert.NotEqual(t, pw, hs)

					t.Logf("hash: %s", hs)
					require.NoError(t, hash.ComparePbkdf2(context.Background(), pw, hs))

					assert.True(t, hasher.Understands(hs))

					mod := make([]byte, len(pw))
					copy(mod, pw)
					mod[len(pw)-1] = ^pw[len(pw)-1]
					require.Error(t, hash.ComparePbkdf2(context.Background(), mod, hs))
				})
			}
		})
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()
	t.Run("unknown", func(t *testing.T) {
		t.Parallel()
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$unknown$12$o6hx.Wog/wvFSkT/Bp/6DOxCtLRTDj7lm9on9suF/WaCGNVHbkfL6")))
	})

	t.Run("bcrypt", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$2a$12$o6hx.Wog/wvFSkT/Bp/6DOxCtLRTDj7lm9on9suF/WaCGNVHbkfL6")))
		assert.Nil(t, hash.CompareBcrypt(context.Background(), []byte("test"), []byte("$2a$12$o6hx.Wog/wvFSkT/Bp/6DOxCtLRTDj7lm9on9suF/WaCGNVHbkfL6")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$2a$12$o6hx.Wog/wvFSkT/Bp/6DOxCtLRTDj7lm9on9suF/WaCGNVHbkfL7")))

		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$2a$15$GRvRO2nrpYTEuPQX6AieaOlZ4.7nMGsXpt.QWMev1zrP86JNspZbO")))
		assert.Nil(t, hash.CompareBcrypt(context.Background(), []byte("test"), []byte("$2a$15$GRvRO2nrpYTEuPQX6AieaOlZ4.7nMGsXpt.QWMev1zrP86JNspZbO")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$2a$15$GRvRO2nrpYTEuPQX6AieaOlZ4.7nMGsXpt.QWMev1zrP86JNspZb1")))
	})

	t.Run("Argon2", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$argon2id$v=19$m=32,t=2,p=4$cm94YnRVOW5jZzFzcVE4bQ$MNzk5BtR2vUhrp6qQEjRNw")))
		assert.Nil(t, hash.CompareArgon2id(context.Background(), []byte("test"), []byte("$argon2id$v=19$m=32,t=2,p=4$cm94YnRVOW5jZzFzcVE4bQ$MNzk5BtR2vUhrp6qQEjRNw")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$argon2id$v=19$m=32,t=2,p=4$cm94YnRVOW5jZzFzcVE4bQ$MNzk5BtR2vUhrp6qQEjRN2")))

		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$argon2i$v=19$m=65536,t=3,p=4$kk51rW/vxIVCYn+EG4kTSg$NyT88uraJ6im6dyha/M5jhXvpqlEdlS/9fEm7ScMb8c")))
		assert.Nil(t, hash.CompareArgon2i(context.Background(), []byte("test"), []byte("$argon2i$v=19$m=65536,t=3,p=4$kk51rW/vxIVCYn+EG4kTSg$NyT88uraJ6im6dyha/M5jhXvpqlEdlS/9fEm7ScMb8c")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$argon2i$v=19$m=65536,t=3,p=4$pZ+27D6B0bCi0DwSmANF1w$4RNCUu4Uyu7eTIvzIdSuKz+I9idJlX/ykn6J10/W0EU")))

		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$argon2id$v=19$m=32,t=5,p=4$cm94YnRVOW5jZzFzcVE4bQ$fBxypOL0nP/zdPE71JtAV71i487LbX3fJI5PoTN6Lp4")))
		assert.Nil(t, hash.CompareArgon2id(context.Background(), []byte("test"), []byte("$argon2id$v=19$m=32,t=5,p=4$cm94YnRVOW5jZzFzcVE4bQ$fBxypOL0nP/zdPE71JtAV71i487LbX3fJI5PoTN6Lp4")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$argon2id$v=19$m=32,t=5,p=4$cm94YnRVOW5jZzFzcVE4bQ$fBxypOL0nP/zdPE71JtAV71i487LbX3fJI5PoTN6Lp5")))
	})

	t.Run("pbkdf2", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha256$i=100000,l=32$1jP+5Zxpxgtee/iPxGgOz0RfE9/KJuDElP1ley4VxXc$QJxzfvdbHYBpydCbHoFg3GJEqMFULwskiuqiJctoYpI")))
		assert.Nil(t, hash.ComparePbkdf2(context.Background(), []byte("test"), []byte("$pbkdf2-sha256$i=100000,l=32$1jP+5Zxpxgtee/iPxGgOz0RfE9/KJuDElP1ley4VxXc$QJxzfvdbHYBpydCbHoFg3GJEqMFULwskiuqiJctoYpI")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha256$i=100000,l=32$1jP+5Zxpxgtee/iPxGgOz0RfE9/KJuDElP1ley4VxXc$QJxzfvdbHYBpydCbHoFg3GJEqMFULwskiuqiJctoYpp")))

		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha512$i=100000,l=32$bdHBpn7OWOivJMVJypy2UqR0UnaD5prQXRZevj/05YU$+wArTfv1a+bNGO1iZrmEdVjhA+lL11wF4/IxpgYfPwc")))
		assert.Nil(t, hash.ComparePbkdf2(context.Background(), []byte("test"), []byte("$pbkdf2-sha512$i=100000,l=32$bdHBpn7OWOivJMVJypy2UqR0UnaD5prQXRZevj/05YU$+wArTfv1a+bNGO1iZrmEdVjhA+lL11wF4/IxpgYfPwc")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha512$i=100000,l=32$bdHBpn7OWOivJMVJypy2UqR0UnaD5prQXRZevj/05YU$+wArTfv1a+bNGO1iZrmEdVjhA+lL11wF4/IxpgYfPww")))

		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha256$1jP+5Zxpxgtee/iPxGgOz0RfE9/KJuDElP1ley4VxXc$QJxzfvdbHYBpydCbHoFg3GJEqMFULwskiuqiJctoYpI")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha256$aaaa$1jP+5Zxpxgtee/iPxGgOz0RfE9/KJuDElP1ley4VxXc$QJxzfvdbHYBpydCbHoFg3GJEqMFULwskiuqiJctoYpI")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha256$i=100000,l=32$1jP+5Zxpxgtee/iPxGgOz0RfE9/KJuDElP1ley4VxXcc$QJxzfvdbHYBpydCbHoFg3GJEqMFULwskiuqiJctoYpI")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha256$i=100000,l=32$1jP+5Zxpxgtee/iPxGgOz0RfE9/KJuDElP1ley4VxXc$QJxzfvdbHYBpydCbHoFg3GJEqMFULwskiuqiJctoYpII")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$pbkdf2-sha512$I=100000,l=32$bdHBpn7OWOivJMVJypy2UqR0UnaD5prQXRZevj/05YU$+wArTfv1a+bNGO1iZrmEdVjhA+lL11wF4/IxpgYfPwc")))
	})

	t.Run("scrypt", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$scrypt$ln=16384,r=8,p=1$2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYE=")))
		assert.Nil(t, hash.CompareScrypt(context.Background(), []byte("test"), []byte("$scrypt$ln=16384,r=8,p=1$2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYE=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$scrypt$ln=16384,r=8,p=1$2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYF=")))

		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$scrypt$2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYE=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$scrypt$ln=16384,r=8,p=1$(2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYE=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$scrypt$ln=16384,r=8,p=1$(2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$(MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYE=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$scrypt$ln=16385,r=8,p=1$2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYE=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("tesu"), []byte("$scrypt$ln=16384,r=8,p=1$2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYE=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("tesu"), []byte("$scrypt$ln=abc,r=8,p=1$2npRo7P03Mt8keSoMbyD/tKFWyUzjiQf2svUaNDSrhA=$MiCzNcIplSMqSBrm4HckjYqYhaVPPjTARTzwB1cVNYE=")))
	})

	t.Run("firescrypt", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, hash.Compare(context.Background(), []byte("8x4WjoDbSxJZdR"), []byte("$firescrypt$ln=14,r=8,p=1$sPtDhWcd1MfdAw==$xbSou7FOl6mChCyzpCPIQ7tku7nsQMTFtyOZSXXd7tjBa4NtimOx7v42Gv2SfzPQu1oxM2/k4SsbOu73wlKe1A==$Bw==$YE0dO4bwD4JnJafh6lZZfkp1MtKzuKAXQcDCJNJNyeCHairWHKENOkbh3dzwaCdizzOspwr/FITUVlnOAwPKyw==")))
		assert.Nil(t, hash.CompareFirebaseScrypt(context.Background(), []byte("8x4WjoDbSxJZdR"), []byte("$firescrypt$ln=14,r=8,p=1$sPtDhWcd1MfdAw==$xbSou7FOl6mChCyzpCPIQ7tku7nsQMTFtyOZSXXd7tjBa4NtimOx7v42Gv2SfzPQu1oxM2/k4SsbOu73wlKe1A==$Bw==$YE0dO4bwD4JnJafh6lZZfkp1MtKzuKAXQcDCJNJNyeCHairWHKENOkbh3dzwaCdizzOspwr/FITUVlnOAwPKyw==")))
		assert.Error(t, hash.Compare(context.Background(), []byte("8x4WjoDbSxJZdR"), []byte("$firescrypt$ln=14,r=8,p=1$sPtDhWcd1MfdAw==$xbSou7FOl6mChCyzpCPIQ7tku7nsQMTFtyOZSXXd7tjBa4NtimOx7v42Gv2SfzPQu1oxM2/k4SsbOu73wlKe1A==$Bw==$YE0dO4bwD4JnJafh6lZZfkp1MtKzuKAXQcDCJNJNyeCHairWHKENOkbh3dzwaCdizzOspwr/FITUVlnOAwPKyc==")))
	})

	t.Run("SSHA", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, hash.Compare(context.Background(), []byte("test123"), []byte("{SSHA}JFZFs0oHzxbMwkSJmYVeI8MnTDy/276a")))
		assert.Nil(t, hash.CompareSSHA(context.Background(), []byte("test123"), []byte("{SSHA}JFZFs0oHzxbMwkSJmYVeI8MnTDy/276a")))
		assert.Error(t, hash.CompareSSHA(context.Background(), []byte("badtest"), []byte("{SSHA}JFZFs0oHzxbMwkSJmYVeI8MnTDy/276a")))
		assert.Error(t, hash.Compare(context.Background(), []byte(""), []byte("{SSHA}tooshort")))

		assert.Nil(t, hash.Compare(context.Background(), []byte("test123"), []byte("{SSHA256}czO44OTV17PcF1cRxWrLZLy9xHd7CWyVYplr1rOhuMlx/7IK")))
		assert.Nil(t, hash.CompareSSHA(context.Background(), []byte("test123"), []byte("{SSHA256}czO44OTV17PcF1cRxWrLZLy9xHd7CWyVYplr1rOhuMlx/7IK")))
		assert.Error(t, hash.CompareSSHA(context.Background(), []byte("badtest"), []byte("{SSHA256}czO44OTV17PcF1cRxWrLZLy9xHd7CWyVYplr1rOhuMlx/7IK")))

		assert.Nil(t, hash.Compare(context.Background(), []byte("test123"), []byte("{SSHA512}xPUl/px+1cG55rUH4rzcwxdOIPSB2TingLpiJJumN2xyDWN4Ix1WQG3ihnvHaWUE8MYNkvMi5rf0C9NYixHsE6Yh59M=")))
		assert.Nil(t, hash.CompareSSHA(context.Background(), []byte("test123"), []byte("{SSHA512}xPUl/px+1cG55rUH4rzcwxdOIPSB2TingLpiJJumN2xyDWN4Ix1WQG3ihnvHaWUE8MYNkvMi5rf0C9NYixHsE6Yh59M=")))
		assert.Error(t, hash.CompareSSHA(context.Background(), []byte("badtest"), []byte("{SSHA512}xPUl/px+1cG55rUH4rzcwxdOIPSB2TingLpiJJumN2xyDWN4Ix1WQG3ihnvHaWUE8MYNkvMi5rf0C9NYixHsE6Yh59M=")))
		assert.Error(t, hash.CompareSSHA(context.Background(), []byte("test123"), []byte("{SSHAnotExistent}xPUl/px+1cG55rUH4rzcwxdOIPSB2TingLpiJJumN2xyDWN4Ix1WQG3ihnvHaWUE8MYNkvMi5rf0C9NYixHsE6Yh59M=")))
	})

	t.Run("sha1", func(t *testing.T) {
		t.Parallel()
		//pf: {SALT}{PASSWORD}
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$2qU2SGWP8viTM1md3FiI3+rjWXQ=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("wrongpass"), []byte("$sha1$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$2qU2SGWP8viTM1md3FiI3+rjWXQ=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("tset"), []byte("$sha1$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$2qU2SGWP8viTM1md3FiI3+rjWXQ=")))
		// wrong salt
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$pf=e1NBTFR9e1BBU1NXT1JEfQ==$cDJvb3ZrZGJ6cQ==$2qU2SGWP8viTM1md3FiI3+rjWXQ=")))
		// salt not encoded
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$pf=e1NBTFR9e1BBU1NXT1JEfQ==$5opmkgz03r$2qU2SGWP8viTM1md3FiI3+rjWXQ=")))
		assert.Nil(t, hash.Compare(context.Background(), []byte("BwS^514g^cv@Z"), []byte("$sha1$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$99h9net4BXl7qdTRaiGUobLROxM=")))
		// no format string
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$pf=$NW9wbWtnejAzcg==$2qU2SGWP8viTM1md3FiI3+rjWXQ=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$$NW9wbWtnejAzcg==$2qU2SGWP8viTM1md3FiI3+rjWXQ=")))
		// wrong number of parameters
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$NW9wbWtnejAzcg==$2qU2SGWP8viTM1md3FiI3+rjWXQ=")))
		// pf: ??staticPrefix??{SALT}{PASSWORD}
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$pf=Pz9zdGF0aWNQcmVmaXg/P3tTQUxUfXtQQVNTV09SRH0=$NW9wbWtnejAzcg==$SAAxMUn7jxckQXkBmsVF0nHwqso=")))
		// pf: {PASSWORD}%%{SALT}
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$pf=e1BBU1NXT1JEfSUle1NBTFR9$NW9wbWtnejAzcg==$YX0AW8/MW5ojUlnzTaR43ucHCog=")))
		// pf: ${PASSWORD}${SALT}$
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha1$pf=JHtQQVNTV09SRH0ke1NBTFR9JA==$NW9wbWtnejAzcg==$iE5n1yjX3oAdxRHwZ4u57I4LpQo=")))
	})

	t.Run("sha256", func(t *testing.T) {
		t.Parallel()
		//pf: {SALT}{PASSWORD}
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha256$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$0gfRVLCvtBCk20udLDEY5vNhujWx7RGjwRIS1ebMsLY=")))
		assert.Nil(t, hash.CompareSHA(context.Background(), []byte("test"), []byte("$sha256$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$0gfRVLCvtBCk20udLDEY5vNhujWx7RGjwRIS1ebMsLY=")))
		assert.Error(t, hash.Compare(context.Background(), []byte("wrongpass"), []byte("$sha256$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$0gfRVLCvtBCk20udLDEY5vNhujWx7RGjwRIS1ebMsLY=")))
		//pf: {SALT}$${PASSWORD}
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha256$pf=e1NBTFR9JCR7UEFTU1dPUkR9$NW9wbWtnejAzcg==$HokCOi9OtiZaZRvnkgemV3B4UUHpI7kA8zq/EZWH2NY=")))
	})

	t.Run("sha512", func(t *testing.T) {
		t.Parallel()
		//pf: {SALT}{PASSWORD}
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha512$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$6ctpVuApMNp0CgBXcdHw/GC562eFEFGr4gpgANX8ZYsX+j5B19IkdmOY2Fytsz3QUwSWdGcUjbqwgJGTH0UYvw==")))
		assert.Nil(t, hash.CompareSHA(context.Background(), []byte("test"), []byte("$sha512$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$6ctpVuApMNp0CgBXcdHw/GC562eFEFGr4gpgANX8ZYsX+j5B19IkdmOY2Fytsz3QUwSWdGcUjbqwgJGTH0UYvw==")))
		assert.Error(t, hash.Compare(context.Background(), []byte("wrongpass"), []byte("$sha512$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$6ctpVuApMNp0CgBXcdHw/GC562eFEFGr4gpgANX8ZYsX+j5B19IkdmOY2Fytsz3QUwSWdGcUjbqwgJGTH0UYvw==")))
		//pf: {SALT}$${PASSWORD}
		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$sha512$pf=e1NBTFR9JCR7UEFTU1dPUkR9$NW9wbWtnejAzcg==$1F9BPW8UtdJkZ9Dhlf+D4X4dJ9xfuH8y04EfuCP2k4aGPPq/aWxU9/xe3LydHmYW1/K3zu3NFO9ETVrZettz3w==")))
	})

	t.Run("sha unknown", func(t *testing.T) {
		t.Parallel()
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$shaNotExistent$pf=e1NBTFR9e1BBU1NXT1JEfQ==$NW9wbWtnejAzcg==$6ctpVuApMNp0CgBXcdHw/GC562eFEFGr4gpgANX8ZYsX+j5B19IkdmOY2Fytsz3QUwSWdGcUjbqwgJGTH0UYvw==")))
	})

	t.Run("md5", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$md5$CY9rzUYh03PK3k6DJie09g==")))
		assert.Nil(t, hash.CompareMD5(context.Background(), []byte("test"), []byte("$md5$CY9rzUYh03PK3k6DJie09g==")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$md5$WhBei51A4TKXgNYuoiZdig==")))
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$md5$Dk/E5LQLsx4yt8QbUbvpdg==")))

		assert.Nil(t, hash.Compare(context.Background(), []byte("ory"), []byte("$md5$ptoWyof5SobW+pbZu2QXoQ==")))
		assert.Nil(t, hash.CompareMD5(context.Background(), []byte("ory"), []byte("$md5$ptoWyof5SobW+pbZu2QXoQ==")))
		assert.Error(t, hash.Compare(context.Background(), []byte("ory"), []byte("$md5$4skj967KRHFsnPFoL5dMMw==")))

		assert.ErrorIs(t, hash.Compare(context.Background(), []byte("ory"), []byte("$md5$$")), hash.ErrInvalidHash)
		assert.Error(t, hash.Compare(context.Background(), []byte("ory"), []byte("$md5$$$")))
		assert.Error(t, hash.Compare(context.Background(), []byte("ory"), []byte("$md5$pf=$$")))
		assert.ErrorIs(t, hash.Compare(context.Background(), []byte("ory"), []byte("$md5$pf=MTIz$Z$")), base64.CorruptInputError(0))
		assert.ErrorIs(t, hash.Compare(context.Background(), []byte("ory"), []byte("$md5$pf=MTIz$Z$")), base64.CorruptInputError(0))
		assert.ErrorIs(t, hash.Compare(context.Background(), []byte("ory"), []byte("$md5$pf=MTIz$MTIz$Z")), base64.CorruptInputError(0))

		assert.Nil(t, hash.Compare(context.Background(), []byte("test"), []byte("$md5$pf=e1NBTFR9e1BBU1NXT1JEfQ==$MTIz$q+RdKCgc+ipCAcm5ChQwlQ=="))) // pf={SALT}{PASSWORD} salt=123
		assert.Error(t, hash.Compare(context.Background(), []byte("test"), []byte("$md5$pf=e1NBTFR9e1BBU1NXT1JEfQ==$MTIz$hh8ZTp1hGPPZQqcr4+UXSQ==")))

		assert.Nil(t, hash.CompareMD5(context.Background(), []byte("test"), []byte("$md5$pf=e1NBTFR9JCR7UEFTU1dPUkR9$MTIzNA==$ud392Z8rfZ+Ou7ZFXYLKbA=="))) // pf={SALT}$${PASSWORD} salt=1234
		assert.Error(t, hash.CompareMD5(context.Background(), []byte("test1"), []byte("$md5$pf=e1NBTFR9JCR7UEFTU1dPUkR9$MTIzNA==$ud392Z8rfZ+Ou7ZFXYLKbA==")))

		assert.Nil(t, hash.CompareMD5(context.Background(), []byte("ory"), []byte("$md5$pf=e1BBU1NXT1JEfXtTQUxUfSQ/$MTIzNDU2Nzg5$8PhwWanVRnpJAFK4NUjR0w=="))) // pf={PASSWORD}{SALT}$? salt=123456789
		assert.Error(t, hash.CompareMD5(context.Background(), []byte("ory1"), []byte("$md5$pf=e1BBU1NXT1JEfXtTQUxUfSQ/$MTIzNDU2Nzg5$8PhwWanVRnpJAFK4NUjR0w==")))
	})
}
