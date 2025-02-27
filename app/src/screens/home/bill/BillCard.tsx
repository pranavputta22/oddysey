import React from 'react';
import { View, Text, StyleSheet, Dimensions, Animated } from 'react-native';
import { colors } from '../../../assets';
import FastImage from 'react-native-fast-image';
import { Bill, formatBillNumber } from '../../../models/Bill';
import { Category, DefaultCategory } from '../../../models/Category';
import { TouchableOpacity } from 'react-native-gesture-handler';
import { SharedElement } from 'react-navigation-shared-element';
import { Config } from '../../../util/Config';
import tinycolor from 'tinycolor2';

type State = {};

type Props = {
  bill: Bill;
  category: Category;
  index: number;
  onPress: () => void;
};

const screenWidth = Dimensions.get('screen').width;
const screenHeight = Dimensions.get('screen').height;

export const BillCardSpecs = {
  width: screenWidth * 0.9,
  height: screenHeight * 0.65,
  externalRadius: 20,
  internalRadius: 10,
  horizontalSpacing: 10,
  verticalSpacing: screenHeight * 0.035,
};

export default class BillCard extends React.PureComponent<Props, State> {
  private imageRef = React.createRef<View>();
  private containerRef = React.createRef<any>();
  private contentRef = React.createRef<any>();

  render() {
    let { bill, index, onPress } = this.props;
    let category = Config.getTopics()[bill.category];
    if (!category) {
      category = DefaultCategory;
      Config.alertUpdateConfig();
    }

    let categoryTextColor = tinycolor(category.bgColor)
      .darken(10)
      .toHexString();

    let categoryColor = tinycolor(category.bgColor).lighten(20).toHexString();

    return (
      <Animated.View
        style={[
          styles.container,
          {
            backgroundColor: category.bgColor,
          },
        ]}
        ref={this.containerRef}
      >
        <TouchableOpacity
          onPress={() => onPress()}
          activeOpacity={1}
          style={{ width: '100%', height: '100%' }}
        >
          <SharedElement
            id={`bill.${bill.number}.photo`}
            style={styles.imageContainer}
          >
            <FastImage style={styles.image} source={{ uri: category.image }} />
          </SharedElement>
          <Animated.View ref={this.contentRef} style={[styles.content]}>
            <View style={{ flex: 1 }}>
              <View style={styles.categoriesContainer}>
                <Text style={styles.number}>{formatBillNumber(bill)}</Text>

                <SharedElement
                  id={`bill.${bill.number}.category`}
                  style={[styles.category, { backgroundColor: category.categoryColor }]}
                >
                  <Text
                    style={[styles.categoryText, { color: category.categoryTextColor }]}
                  >
                    {bill.category}
                  </Text>
                </SharedElement>
              </View>
              <Text
                style={styles.title}
                numberOfLines={2}
                adjustsFontSizeToFit={true}
              >
                {bill.title}
              </Text>
              <Text ellipsizeMode="tail" style={styles.synopsis}>
                {bill.short_summary}
              </Text>
            </View>
          </Animated.View>
        </TouchableOpacity>
      </Animated.View>
    );
  }
}

const styles = StyleSheet.create({
  container: {
    height: BillCardSpecs.height,
    width: BillCardSpecs.width,
    borderRadius: BillCardSpecs.externalRadius,
    alignSelf: 'center',
    marginHorizontal: BillCardSpecs.horizontalSpacing,
    marginBottom: BillCardSpecs.verticalSpacing,
  },
  touchableContainer: {
    width: '100%',
    height: '100%',
    alignSelf: 'center',
    borderRadius: BillCardSpecs.externalRadius,
    shadowColor: 'black',
    shadowOpacity: 1,
    shadowRadius: 5,
  },
  imageContainer: {
    width: '100%',
    alignItems: 'center',
    justifyContent: 'center',
    flex: 1,
  },
  image: {
    width: '100%',
    height: '100%',
    resizeMode: 'cover',
    flex: 1,
    borderTopLeftRadius: BillCardSpecs.externalRadius,
    borderTopRightRadius: BillCardSpecs.externalRadius,
  },
  content: {
    margin: '5%',
    backgroundColor: 'white',
    borderRadius: BillCardSpecs.internalRadius,
    paddingVertical: '5%',
    paddingHorizontal: '7.5%',
    flex: 2,
    shadowColor: 'rgba(0, 0, 0, 0.5)',
    shadowRadius: 10,
    shadowOpacity: 0.2,
    shadowOffset: { width: 0, height: 4 },
    zIndex: 100,
  },
  number: {
    fontFamily: 'Roboto-Light',
    fontWeight: '400',
    fontSize: 16,
    color: colors.blueGray,
  },
  title: {
    fontSize: 20,
    fontFamily: 'Futura',
    fontWeight: '700',
    marginTop: '5%',
  },
  categoriesContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  category: {
    justifyContent: 'center',
    alignItems: 'center',
    padding: '2%',
    paddingHorizontal: '4%',
    borderRadius: 10,
  },
  categoryText: {
    fontFamily: 'Roboto-Thin',
    fontWeight: '800',
  },
  header: {
    fontSize: 16,
    fontWeight: 'bold',
    marginTop: '10%',
  },
  synopsis: {
    flex: 1,
    flexWrap: 'wrap',
    textAlignVertical: 'center',
    marginTop: '5%',
    fontSize: 15,
    fontFamily: 'Futura',
  },
  heartButton: {
    position: 'absolute',
    margin: '5%',
    backgroundColor: 'white',
    justifyContent: 'center',
    alignItems: 'center',
    borderRadius: 40,
  },
  heartButtonIcon: {},
});
