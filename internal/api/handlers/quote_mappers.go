// Package handlers contains the handlers for the API
package handlers

import (
	"log"

	"github.com/nsvirk/moneybotsapi/internal/models"
)

func mapTickToQuoteData(tick *models.TickerData) interface{} {
	ohlc, err := tick.GetOHLC()
	if err != nil {
		log.Printf("Error getting OHLC data: %v", err)
		ohlc = models.TickerDataOHLC{} // Use default OHLC
	}

	depth, err := tick.GetDepth()
	if err != nil {
		log.Printf("Error getting Depth data: %v", err)
		depth = models.TickerDataDepth{} // Use default Depth
	}

	return models.QuoteData{
		Instrument:         tick.Instrument,
		Mode:               tick.Mode,
		InstrumentToken:    tick.InstrumentToken,
		IsTradable:         tick.IsTradable,
		IsIndex:            tick.IsIndex,
		Timestamp:          tick.Timestamp.Format("2006-01-02 15:04:05"),
		LastTradeTime:      tick.LastTradeTime.Format("2006-01-02 15:04:05"),
		LastPrice:          tick.LastPrice,
		LastTradedQuantity: tick.LastTradedQuantity,
		TotalBuyQuantity:   tick.TotalBuyQuantity,
		TotalSellQuantity:  tick.TotalSellQuantity,
		VolumeTraded:       tick.VolumeTraded,
		// TotalBuy:           tick.TotalBuy,
		// TotalSell:          tick.TotalSell,
		AverageTradePrice: tick.AverageTradePrice,
		OI:                tick.OI,
		OIDayHigh:         tick.OIDayHigh,
		OIDayLow:          tick.OIDayLow,
		NetChange:         tick.NetChange,
		OHLC:              mapOHLC(ohlc),
		Depth:             mapDepth(depth),
		UpdatedAt:         tick.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func mapTickToOHLCData(tick *models.TickerData) interface{} {
	ohlc, err := tick.GetOHLC()
	if err != nil {
		log.Printf("Error getting OHLC data: %v", err)
	}

	return models.OHLCData{
		InstrumentToken:   tick.InstrumentToken,
		LastPrice:         tick.LastPrice,
		VolumeTraded:      tick.VolumeTraded,
		AverageTradePrice: tick.AverageTradePrice,
		Timestamp:         tick.Timestamp.Format("2006-01-02 15:04:05"),
		LastTradeTime:     tick.LastTradeTime.Format("2006-01-02 15:04:05"),
		OHLC:              mapOHLC(ohlc),
		UpdatedAt:         tick.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func mapTickToLTPData(tick *models.TickerData) interface{} {
	return models.LTPData{
		InstrumentToken: tick.InstrumentToken,
		LastPrice:       tick.LastPrice,
		Timestamp:       tick.Timestamp.Format("2006-01-02 15:04:05"),
		UpdatedAt:       tick.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func mapOHLC(ohlc models.TickerDataOHLC) models.OHLC {
	return models.OHLC(ohlc)
}

func mapDepth(depth models.TickerDataDepth) models.Depth {
	return models.Depth{
		Buy:  mapDepthItems(depth.Buy),
		Sell: mapDepthItems(depth.Sell),
	}
}

func mapDepthItems(items [5]models.TickerDataDepthItem) [5]models.DepthItem {
	var mappedItems [5]models.DepthItem
	for i, item := range items {
		mappedItems[i] = models.DepthItem(item)
	}
	return mappedItems
}
