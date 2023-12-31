package service

import (
	"context"
	"fmt"
	"strconv"

	model "github.com/gamepkw/transactions-banking-microservice/internal/models"
	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

func (a *transactionService) Deposit(c context.Context, tr *model.Transaction) (err error) {
	ctx, cancel := context.WithTimeout(c, a.contextTimeout)
	defer cancel()

	acc, err := a.restGetAccountByAccountNo(ctx, tr.Account.AccountNo)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error get account data: %s", tr.Account.AccountNo))
	}

	var limitAmount string

	cacheKey := "min_deposit_amount"
	limitAmount, err = a.redis.Get(cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			fmt.Printf("redis key %s is missing", cacheKey)
			limitAmount, _ = a.transactionRepo.GetTransactionConfig(ctx, cacheKey)
		}
		if err := a.redis.Set(cacheKey, limitAmount, -1).Err(); err != nil {
			return err
		}
	}

	floatLimitAmount, _ := strconv.ParseFloat(limitAmount, 64)

	if floatLimitAmount > tr.Amount {
		return model.ErrMinimumDeposit
	}
	tr.Total = tr.Amount
	acc.Balance += tr.Total

	if err = a.restUpdateAccount(ctx, *acc); err != nil {
		return errors.Wrap(err, fmt.Sprintf("error update account: %s", acc.AccountNo))
	}

	if err = a.createTransaction(ctx, tr); err != nil {
		return errors.Wrap(err, fmt.Sprintf("error create transaction"))
	}

	tr.Account = *acc

	go a.addTransactionNotiToQueue(ctx, *tr, acc.Balance)

	return nil

}
