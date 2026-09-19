package auth

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	current time.Time
}

func (c *fakeClock) now() time.Time { return c.current }

func (c *fakeClock) advance(d time.Duration) { c.current = c.current.Add(d) }

func newTestLimiter(limit attemptLimit) (*attemptLimiter, *fakeClock) {
	clock := &fakeClock{current: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	limiter := newAttemptLimiter(limit)
	limiter.now = clock.now

	return limiter, clock
}

func exhaust(t *testing.T, limiter *attemptLimiter, key string) {
	t.Helper()

	for range limiter.limit.failures {
		require.True(t, limiter.allow(key))
	}
}

func TestAttemptLimiterBlocksAfterLimit(t *testing.T) {
	t.Parallel()

	limiter, clock := newTestLimiter(loginLimit)
	exhaust(t, limiter, "ivanov.i")

	assert.False(t, limiter.allow("ivanov.i"))
	assert.True(t, limiter.allow("petrov.p"))

	clock.advance(loginLimit.block)

	assert.True(t, limiter.allow("ivanov.i"))
	assert.False(t, limiter.allow("ivanov.i"))
}

func TestAttemptLimiterResetOnSuccess(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(loginLimit)

	for range loginLimit.failures - 1 {
		require.True(t, limiter.allow("ivanov.i"))
	}

	limiter.reset("ivanov.i")
	exhaust(t, limiter, "ivanov.i")
}

func TestAttemptLimiterStartsNewWindow(t *testing.T) {
	t.Parallel()

	limiter, clock := newTestLimiter(loginLimit)

	for range loginLimit.failures - 1 {
		require.True(t, limiter.allow("ivanov.i"))
	}

	clock.advance(loginLimit.window)
	exhaust(t, limiter, "ivanov.i")
}

func TestAttemptLimiterRefundDoesNotCountSuccess(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(ipLimit)

	for range 10 * ipLimit.failures {
		require.True(t, limiter.allow("192.0.2.1"))
		limiter.refund("192.0.2.1")
	}

	exhaust(t, limiter, "192.0.2.1")
	assert.False(t, limiter.allow("192.0.2.1"))
}

func TestAttemptLimiterKeepsBlockedKeysWhenFull(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(loginLimit)
	exhaust(t, limiter, "ivanov.i")

	for i := range maxTrackedAttempts {
		require.True(t, limiter.allow("flood-"+strconv.Itoa(i)))
	}

	assert.LessOrEqual(t, len(limiter.entries), maxTrackedAttempts)
	assert.False(t, limiter.allow("ivanov.i"))
}

func failLogin(t *testing.T, svc *Service, attempt LoginAttempt, times int) {
	t.Helper()

	attempt.Password = "wrong"

	for range times {
		_, err := svc.LoginFrom(t.Context(), attempt)
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
}

func TestLoginBlockedAfterRepeatedFailures(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	require.NoError(t, svc.EnsureAdmin(t.Context(), adminConfig()))

	failLogin(t, svc, LoginAttempt{Login: "admin"}, loginLimit.failures)

	_, err := svc.Login(t.Context(), "admin", "secret")
	require.ErrorIs(t, err, ErrTooManyAttempts)
}

func TestLoginSuccessClearsFailures(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	for range 2 * loginLimit.failures {
		failLogin(t, svc, LoginAttempt{Login: "admin"}, 1)

		_, err := svc.Login(ctx, "admin", "secret")
		require.NoError(t, err)
	}
}

func TestKnownDeviceIsNotLockedOutByOthers(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	first, err := svc.LoginFrom(ctx, LoginAttempt{Login: "admin", Password: "secret", IP: "192.0.2.1"})
	require.NoError(t, err)
	assert.False(t, first.KnownDevice)

	token, err := svc.RememberDevice(ctx, first.Session.UserID)
	require.NoError(t, err)

	failLogin(t, svc, LoginAttempt{Login: "admin", IP: "192.0.2.1"}, loginLimit.failures)

	_, err = svc.LoginFrom(ctx, LoginAttempt{Login: "admin", Password: "secret", IP: "192.0.2.1"})
	require.ErrorIs(t, err, ErrTooManyAttempts)

	_, err = svc.LoginFrom(ctx, LoginAttempt{Login: "admin", Password: "secret", IP: "192.0.2.1", DeviceToken: "forged"})
	require.ErrorIs(t, err, ErrTooManyAttempts)

	known, err := svc.LoginFrom(ctx, LoginAttempt{Login: "admin", Password: "secret", IP: "192.0.2.1", DeviceToken: token})
	require.NoError(t, err)
	assert.True(t, known.KnownDevice)
}

func TestKnownDeviceHasOwnLimit(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	session, err := svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)

	token, err := svc.RememberDevice(ctx, session.UserID)
	require.NoError(t, err)

	failLogin(t, svc, LoginAttempt{Login: "admin", DeviceToken: token}, deviceLimit.failures)

	_, err = svc.LoginFrom(ctx, LoginAttempt{Login: "admin", Password: "secret", DeviceToken: token})
	require.ErrorIs(t, err, ErrTooManyAttempts)

	_, err = svc.Login(ctx, "admin", "secret")
	require.NoError(t, err)
}

func TestDeviceOfAnotherUserIsNotTrusted(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	student, err := svc.CreateUser(ctx, NewUser{Role: RoleStudent, Name: Name{Last: "Петров", First: "Пётр"}})
	require.NoError(t, err)

	token, err := svc.RememberDevice(ctx, student.User.ID)
	require.NoError(t, err)

	failLogin(t, svc, LoginAttempt{Login: "admin", DeviceToken: token}, loginLimit.failures)

	_, err = svc.Login(ctx, "admin", "secret")
	require.ErrorIs(t, err, ErrTooManyAttempts)
}

func TestLoginBlockedByIPAcrossLogins(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	require.NoError(t, svc.EnsureAdmin(ctx, adminConfig()))

	for i := range ipLimit.failures {
		failLogin(t, svc, LoginAttempt{Login: "user-" + strconv.Itoa(i), IP: "192.0.2.1"}, 1)
	}

	_, err := svc.LoginFrom(ctx, LoginAttempt{Login: "admin", Password: "secret", IP: "192.0.2.1"})
	require.ErrorIs(t, err, ErrTooManyAttempts)

	_, err = svc.LoginFrom(ctx, LoginAttempt{Login: "admin", Password: "secret", IP: "192.0.2.2"})
	require.NoError(t, err)
}

func TestResetPasswordForgetsDevices(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, time.Hour)
	ctx := t.Context()

	created, err := svc.CreateUser(ctx, NewUser{Role: RoleStudent, Name: Name{Last: "Петров", First: "Пётр"}})
	require.NoError(t, err)

	token, err := svc.RememberDevice(ctx, created.User.ID)
	require.NoError(t, err)

	password, err := svc.ResetPassword(ctx, created.User.ID)
	require.NoError(t, err)

	result, err := svc.LoginFrom(ctx, LoginAttempt{Login: created.User.Login, Password: password, DeviceToken: token})
	require.NoError(t, err)
	assert.False(t, result.KnownDevice)
}
