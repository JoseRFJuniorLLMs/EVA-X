package googlefit

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/fitness/v1"
	"google.golang.org/api/option"
)

type Service struct {
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{ctx: ctx}
}

type HealthData struct {
	Steps         int64
	HeartRate     float64
	Calories      int64
	Distance      float64
	SleepHours    float64
	Weight        float64
	BloodPressure string
	SpO2          int64
}

// GetAllHealthData gets comprehensive health data from Google Fit
func (s *Service) GetAllHealthData(accessToken string) (*HealthData, error) {
	data := &HealthData{}

	// Get steps
	steps, _ := s.GetStepsToday(accessToken)
	data.Steps = steps

	// Get heart rate
	hr, _ := s.GetHeartRate(accessToken)
	data.HeartRate = hr

	// Get calories
	calories, _ := s.GetCaloriesToday(accessToken)
	data.Calories = calories

	// Get distance
	distance, _ := s.GetDistanceToday(accessToken)
	data.Distance = distance

	// Get weight (most recent)
	weight, _ := s.GetWeight(accessToken)
	data.Weight = weight

	return data, nil
}

// GetStepsToday gets step count for today
func (s *Service) GetStepsToday(accessToken string) (int64, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := fitness.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return 0, fmt.Errorf("unable to create fitness client: %v", err)
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	dataSourceID := "derived:com.google.step_count.delta:com.google.android.gms:estimated_steps"
	datasetID := fmt.Sprintf("%d000000-%d000000", startOfDay.Unix(), now.Unix())

	dataset, err := srv.Users.DataSources.Datasets.Get("me", dataSourceID, datasetID).Do()
	if err != nil {
		return 0, fmt.Errorf("unable to get steps: %v", err)
	}

	var totalSteps int64
	for _, point := range dataset.Point {
		for _, value := range point.Value {
			totalSteps += value.IntVal
		}
	}

	return totalSteps, nil
}

// GetHeartRate gets recent heart rate data
func (s *Service) GetHeartRate(accessToken string) (float64, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := fitness.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return 0, fmt.Errorf("unable to create fitness client: %v", err)
	}

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	dataSourceID := "derived:com.google.heart_rate.bpm:com.google.android.gms:merge_heart_rate_bpm"
	datasetID := fmt.Sprintf("%d000000-%d000000", oneHourAgo.Unix(), now.Unix())

	dataset, err := srv.Users.DataSources.Datasets.Get("me", dataSourceID, datasetID).Do()
	if err != nil {
		return 0, fmt.Errorf("unable to get heart rate: %v", err)
	}

	if len(dataset.Point) == 0 {
		return 0, fmt.Errorf("no heart rate data available")
	}

	// Get most recent value
	lastPoint := dataset.Point[len(dataset.Point)-1]
	if len(lastPoint.Value) > 0 {
		return lastPoint.Value[0].FpVal, nil
	}

	return 0, fmt.Errorf("no heart rate value found")
}

// GetCaloriesToday gets calories burned today
func (s *Service) GetCaloriesToday(accessToken string) (int64, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := fitness.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return 0, err
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	dataSourceID := "derived:com.google.calories.expended:com.google.android.gms:merge_calories_expended"
	datasetID := fmt.Sprintf("%d000000-%d000000", startOfDay.Unix(), now.Unix())

	dataset, err := srv.Users.DataSources.Datasets.Get("me", dataSourceID, datasetID).Do()
	if err != nil {
		return 0, err
	}

	var totalCalories float64
	for _, point := range dataset.Point {
		for _, value := range point.Value {
			totalCalories += value.FpVal
		}
	}

	return int64(totalCalories), nil
}

// GetDistanceToday gets distance traveled today in meters
func (s *Service) GetDistanceToday(accessToken string) (float64, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := fitness.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return 0, err
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	dataSourceID := "derived:com.google.distance.delta:com.google.android.gms:merge_distance_delta"
	datasetID := fmt.Sprintf("%d000000-%d000000", startOfDay.Unix(), now.Unix())

	dataset, err := srv.Users.DataSources.Datasets.Get("me", dataSourceID, datasetID).Do()
	if err != nil {
		return 0, err
	}

	var totalDistance float64
	for _, point := range dataset.Point {
		for _, value := range point.Value {
			totalDistance += value.FpVal
		}
	}

	// Convert meters to kilometers
	return totalDistance / 1000.0, nil
}

// GetWeight gets most recent weight measurement
func (s *Service) GetWeight(accessToken string) (float64, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := fitness.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return 0, err
	}

	now := time.Now()
	oneMonthAgo := now.AddDate(0, -1, 0)

	dataSourceID := "derived:com.google.weight:com.google.android.gms:merge_weight"
	datasetID := fmt.Sprintf("%d000000-%d000000", oneMonthAgo.Unix(), now.Unix())

	dataset, err := srv.Users.DataSources.Datasets.Get("me", dataSourceID, datasetID).Do()
	if err != nil {
		return 0, err
	}

	if len(dataset.Point) == 0 {
		return 0, fmt.Errorf("no weight data available")
	}

	// Get most recent value
	lastPoint := dataset.Point[len(dataset.Point)-1]
	if len(lastPoint.Value) > 0 {
		return lastPoint.Value[0].FpVal, nil
	}

	return 0, fmt.Errorf("no weight value found")
}
