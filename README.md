Calculate a person or companies availability by reverse engineering how much open time from their calendar services (supports google, calendly, cal.com)

cli tool written in golang, has two options:

1. get all calendly links from website  ./busybody -site startup.com
2. check a calendar directly ./busybody -calendar cal.com/person

Default behavior is to evaluate the most recent available work week (M-F) to determine how much they are booked up and how much free based on calendar availability to generate a busyness score.






