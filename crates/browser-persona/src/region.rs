use serde::{Deserialize, Serialize};

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RegionPreset {
    #[default]
    Native,
    Custom,
    Shanghai,
    NewYork,
    Berlin,
    Moscow,
}

impl RegionPreset {
    #[must_use]
    pub const fn values(self) -> Option<RegionPresetValues> {
        match self {
            Self::Native | Self::Custom => None,
            Self::Shanghai => Some(RegionPresetValues {
                locale_tag: "zh-CN",
                timezone_id: "Asia/Shanghai",
                geolocation: Geolocation::new(31_230_400, 121_473_700, 100),
            }),
            Self::NewYork => Some(RegionPresetValues {
                locale_tag: "en-US",
                timezone_id: "America/New_York",
                geolocation: Geolocation::new(40_712_800, -74_006_000, 100),
            }),
            Self::Berlin => Some(RegionPresetValues {
                locale_tag: "de-DE",
                timezone_id: "Europe/Berlin",
                geolocation: Geolocation::new(52_520_000, 13_405_000, 100),
            }),
            Self::Moscow => Some(RegionPresetValues {
                locale_tag: "ru-RU",
                timezone_id: "Europe/Moscow",
                geolocation: Geolocation::new(55_755_800, 37_617_300, 100),
            }),
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Geolocation {
    pub latitude_e6: i32,
    pub longitude_e6: i32,
    pub accuracy_meters: u16,
}

impl Geolocation {
    #[must_use]
    pub const fn new(latitude_e6: i32, longitude_e6: i32, accuracy_meters: u16) -> Self {
        Self {
            latitude_e6,
            longitude_e6,
            accuracy_meters,
        }
    }

    #[must_use]
    pub fn latitude(self) -> f64 {
        f64::from(self.latitude_e6) / 1_000_000.0
    }

    #[must_use]
    pub fn longitude(self) -> f64 {
        f64::from(self.longitude_e6) / 1_000_000.0
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct RegionPresetValues {
    pub locale_tag: &'static str,
    pub timezone_id: &'static str,
    pub geolocation: Geolocation,
}

#[cfg(test)]
mod tests {
    use super::{Geolocation, RegionPreset};

    #[test]
    fn named_presets_are_complete_and_stable() {
        let berlin = RegionPreset::Berlin.values().unwrap();
        assert_eq!(berlin.locale_tag, "de-DE");
        assert_eq!(berlin.timezone_id, "Europe/Berlin");
        assert_eq!(
            berlin.geolocation,
            Geolocation::new(52_520_000, 13_405_000, 100)
        );
        assert_eq!(berlin.geolocation.latitude(), 52.52);
        assert_eq!(berlin.geolocation.longitude(), 13.405);
    }
}
