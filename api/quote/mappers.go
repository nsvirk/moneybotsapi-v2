package quote

import (
	"log"

	"github.com/nsvirk/moneybotsapi/services/ticker"
)

func mapTickToQuoteData(tick *ticker.TickerData) interface{} {
	ohlc, err := tick.GetOHLC()
	if err != nil {
		log.Printf("Error getting OHLC data: %v", err)
		ohlc = ticker.TickerDataOHLC{} // Use default OHLC
	}

	depth, err := tick.GetDepth()
	if err != nil {
		log.Printf("Error getting Depth data: %v", err)
		depth = ticker.TickerDataDepth{} // Use default Depth
	}

	return QuoteData{
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

func mapTickToOHLCData(tick *ticker.TickerData) interface{} {
	ohlc, err := tick.GetOHLC()
	if err != nil {
		log.Printf("Error getting OHLC data: %v", err)
	}

	return OHLCData{
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

func mapTickToLTPData(tick *ticker.TickerData) interface{} {
	return LTPData{
		InstrumentToken: tick.InstrumentToken,
		LastPrice:       tick.LastPrice,
		Timestamp:       tick.Timestamp.Format("2006-01-02 15:04:05"),
		UpdatedAt:       tick.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func mapOHLC(ohlc ticker.TickerDataOHLC) OHLC {
	return OHLC{
		Open:  ohlc.Open,
		High:  ohlc.High,
		Low:   ohlc.Low,
		Close: ohlc.Close,
	}
}

func mapDepth(depth ticker.TickerDataDepth) Depth {
	return Depth{
		Buy:  mapDepthItems(depth.Buy),
		Sell: mapDepthItems(depth.Sell),
	}
}

func mapDepthItems(items [5]ticker.TickerDataDepthItem) [5]DepthItem {
	var mappedItems [5]DepthItem
	for i, item := range items {
		mappedItems[i] = DepthItem{
			Price:    item.Price,
			Quantity: item.Quantity,
			Orders:   item.Orders,
		}
	}
	return mappedItems
}
